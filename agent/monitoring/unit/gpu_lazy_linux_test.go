//go:build linux

package monitoring

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGPUDisabledDoesNotInitializeVendor(t *testing.T) {
	old := flags.EnableGPU
	flags.EnableGPU = false
	defer func() { flags.EnableGPU = old }()
	_, _ = GetDetailedGPUHost()
	_, _ = GetDetailedGPUInfo()
	if vendorType != 0 {
		t.Fatalf("vendor initialized with GPU disabled: %d", vendorType)
	}
}

func TestGPUModelIsCached(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "lspci")
	t.Setenv("PATH", dir)
	if err := os.WriteFile(tool, []byte("#!/bin/sh\necho '00:00 VGA compatible controller: NVIDIA first'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	first := GpuName()
	if first != "NVIDIA first" {
		t.Fatalf("first=%q", first)
	}
	if err := os.WriteFile(tool, []byte("#!/bin/sh\necho '00:00 VGA compatible controller: NVIDIA second'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if next := GpuName(); next != first {
		t.Fatalf("static GPU model re-polled: %q -> %q", first, next)
	}
}
