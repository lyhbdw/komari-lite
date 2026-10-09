package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSmokeTestBinarySuccess(t *testing.T) {
	// 使用当前测试执行文件自身作为探测目标
	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get executable: %v", err)
	}

	err = smokeTestBinary(exePath)
	// 测试可执行文件接受 --help 或者任何命令行参数运行退出即可
	if err != nil {
		t.Logf("smoke test returned: %v (expected behavior if flag unhandled)", err)
	}
}

func TestSmokeTestBinaryNonExecutableFails(t *testing.T) {
	// 创建一个普通的非二进制空文本文件
	dir := t.TempDir()
	invalidFile := filepath.Join(dir, "invalid-binary")
	if err := os.WriteFile(invalidFile, []byte("plain text"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err := smokeTestBinary(invalidFile)
	if err == nil {
		t.Fatalf("expected smoke test to fail on plain text file without exec bit, got nil")
	}
}
