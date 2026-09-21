package admin

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/web/api"
)

// copyFile 复制单个文件到目标路径（会确保父目录存在）
func copyFile(srcPath, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory: %v", err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer src.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer dest.Close()

	if _, err = io.Copy(dest, src); err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}
	return nil
}

// walkDirToZip 将 contentDir 的内容写入 zip writer。
// 注意：contentDir 不应包含被 walk 的 zip 文件本身。
func walkDirToZip(zipWriter *zip.Writer, contentDir string) error {
	return filepath.Walk(contentDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(contentDir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		zipPath := filepath.ToSlash(rel)
		if info.IsDir() {
			_, err := zipWriter.CreateHeader(&zip.FileHeader{
				Name:     zipPath + "/",
				Method:   zip.Deflate,
				Modified: info.ModTime(),
			})
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		w, err := zipWriter.CreateHeader(&zip.FileHeader{
			Name:     zipPath,
			Method:   zip.Deflate,
			Modified: info.ModTime(),
		})
		if err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		return err
	})
}

// writeBackupMarkup 追加备份标记文件到 zip。
func writeBackupMarkup(zipWriter *zip.Writer) error {
	now := time.Now().UTC()
	markupContent := "此文件为 Komari 备份标记文件，请勿删除。\nThis is a Komari backup markup file, please do not delete.\n\n备份时间 / Backup Time: " + now.Format(time.RFC3339Nano)
	markupWriter, err := zipWriter.CreateHeader(&zip.FileHeader{
		Name:     "komari-backup-markup",
		Method:   zip.Deflate,
		Modified: now,
	})
	if err != nil {
		return err
	}
	_, err = markupWriter.Write([]byte(markupContent))
	return err
}

// backupSQLiteTo 使用 SQLite VACUUM INTO 将当前数据库一致性备份到指定路径。
// destDBPath 会先删除以防 SQLite 报 "output file already exists"。
func backupSQLiteTo(destDBPath string) error {
	if err := os.MkdirAll(filepath.Dir(destDBPath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory for db: %v", err)
	}

	// 确保目标文件不存在（VACUUM INTO 要求目标文件不存在）
	_ = os.Remove(destDBPath)

	db := dbcore.GetDBInstance()
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying database connection: %v", err)
	}

	// Windows 下 VACUUM INTO 传绝对路径时，统一使用正斜杠避免路径解析歧义
	safePath := filepath.ToSlash(destDBPath)
	safePath = strings.ReplaceAll(safePath, "'", "''")
	vacuumSQL := fmt.Sprintf("VACUUM INTO '%s'", safePath)
	if _, err = sqlDB.Exec(vacuumSQL); err != nil {
		return fmt.Errorf("sqlite VACUUM INTO failed: %v", err)
	}
	return nil
}

// createBackupArchive creates and durably publishes a backup archive on the data volume.
// It intentionally does not write the archive to the HTTP response, so callers can
// run it asynchronously without waiting for a large ZIP download through a proxy.
func createBackupArchive() (string, string, error) {
	backupDir := filepath.Join(".", "data", "backup")
	tempRoot := filepath.Join(".", "data", ".komari-backup-tmp")
	if err := os.MkdirAll(tempRoot, 0o700); err != nil {
		return "", "", fmt.Errorf("error creating backup work directory: %w", err)
	}
	tempDir, err := os.MkdirTemp(tempRoot, "komari-backup-*")
	if err != nil {
		return "", "", fmt.Errorf("error creating temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	contentDir := filepath.Join(tempDir, "content")
	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		return "", "", fmt.Errorf("error creating content directory: %w", err)
	}
	if err := copyWhitelistedFiles(contentDir); err != nil {
		return "", "", fmt.Errorf("error copying data to temp: %w", err)
	}

	destDB := filepath.Join(contentDir, "komari.db")
	dbFilePath := flags.DatabaseFile
	if flags.IsSQLite() {
		if err := backupSQLiteTo(destDB); err != nil {
			return "", "", fmt.Errorf("error backing up sqlite database: %w", err)
		}
	} else if dbFilePath != "" {
		if _, err := os.Stat(dbFilePath); err == nil {
			if err := copyFile(dbFilePath, destDB); err != nil {
				return "", "", fmt.Errorf("error copying database file: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return "", "", fmt.Errorf("error stating database file: %w", err)
		}
	}

	tempZipPath := filepath.Join(tempDir, "output.zip")
	tempZip, err := os.Create(tempZipPath)
	if err != nil {
		return "", "", fmt.Errorf("error creating temp zip: %w", err)
	}
	zipWriter := zip.NewWriter(tempZip)
	if err := walkDirToZip(zipWriter, contentDir); err != nil {
		_ = zipWriter.Close()
		_ = tempZip.Close()
		return "", "", fmt.Errorf("error archiving temp folder: %w", err)
	}
	if err := writeBackupMarkup(zipWriter); err != nil {
		_ = zipWriter.Close()
		_ = tempZip.Close()
		return "", "", fmt.Errorf("error writing backup markup: %w", err)
	}
	if err := zipWriter.Close(); err != nil {
		_ = tempZip.Close()
		return "", "", fmt.Errorf("error finalizing zip: %w", err)
	}
	if err := tempZip.Close(); err != nil {
		return "", "", fmt.Errorf("error closing temp zip: %w", err)
	}

	archiveName := "database-maintenance-backup.zip"
	archivePath := filepath.Join(backupDir, archiveName)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", "", fmt.Errorf("error creating backup directory: %w", err)
	}
	archiveTemp, err := os.CreateTemp(backupDir, ".backup-*.tmp")
	if err != nil {
		return "", "", fmt.Errorf("error creating archive temp file: %w", err)
	}
	archiveTempPath := archiveTemp.Name()
	if err := archiveTemp.Close(); err != nil {
		_ = os.Remove(archiveTempPath)
		return "", "", fmt.Errorf("error closing archive temp file: %w", err)
	}
	defer os.Remove(archiveTempPath)
	if err := copyFile(tempZipPath, archiveTempPath); err != nil {
		return "", "", fmt.Errorf("error archiving backup: %w", err)
	}
	if err := os.Rename(archiveTempPath, archivePath); err != nil {
		return "", "", fmt.Errorf("error publishing backup archive: %w", err)
	}
	// Keep one deterministic maintenance backup instead of accumulating large ZIPs.
	entries, _ := os.ReadDir(backupDir)
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == archiveName ||
			(!strings.HasPrefix(entry.Name(), "database-maintenance-backup-") &&
				!strings.HasPrefix(entry.Name(), "backup-")) {
			continue
		}
		_ = os.Remove(filepath.Join(backupDir, entry.Name()))
	}
	return archivePath, archiveName, nil
}

// DownloadBackup preserves the manual full ZIP download endpoint.
func DownloadBackup(c *gin.Context) {
	archivePath, archiveName, err := createBackupArchive()
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	zipReader, err := os.Open(archivePath)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, fmt.Sprintf("error reading backup archive: %v", err))
		return
	}
	defer zipReader.Close()
	c.Writer.Header().Set("Content-Type", "application/zip")
	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", archiveName))
	http.ServeContent(c.Writer, c.Request, archiveName, time.Now(), zipReader)
}
