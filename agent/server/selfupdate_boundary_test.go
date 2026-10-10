package server

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lyhbdw/komari-lite/agent/internal/boundedexec"
)

func checksum(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestStageBinaryLimitHashPermissionsAndCleanup(t *testing.T) {
	for _, tc := range []struct {
		name, data, sum string
		limit           int64
		reader          io.Reader
		wantErr         bool
	}{
		{name: "exact", data: "1234", sum: checksum("1234"), limit: 4},
		{name: "too-large", data: "12345", sum: checksum("12345"), limit: 4, wantErr: true},
		{name: "mismatch", data: "1234", sum: checksum("nope"), limit: 4, wantErr: true},
		{name: "empty", sum: checksum(""), limit: 4, wantErr: true},
		{name: "read-error", sum: checksum("1234"), limit: 4, reader: brokenReader{}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			reader := tc.reader
			if reader == nil {
				reader = strings.NewReader(tc.data)
			}
			path, err := stageBinary(reader, dir, tc.sum, tc.limit)
			if (err != nil) != tc.wantErr {
				t.Fatalf("stage error=%v; wantErr=%v", err, tc.wantErr)
			}
			entries, _ := os.ReadDir(dir)
			if tc.wantErr {
				if path != "" || len(entries) != 0 {
					t.Fatalf("failed stage leaked file: %q %v", path, entries)
				}
				return
			}
			if filepath.Dir(path) != dir {
				t.Fatal("stage not in executable directory")
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != tc.data {
				t.Fatalf("staged=%q,%v", data, err)
			}
			info, _ := os.Stat(path)
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0755 {
				t.Fatalf("mode=%v", info.Mode())
			}
		})
	}
}

func TestSmokeTestRejectsOutputFlood(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	path := filepath.Join(t.TempDir(), "noisy-helper")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s' '"+strings.Repeat("x", 65537)+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := smokeTestBinary(path); !errors.Is(err, boundedexec.ErrOutputLimit) {
		t.Fatalf("flood error=%v; want output limit", err)
	}
}
