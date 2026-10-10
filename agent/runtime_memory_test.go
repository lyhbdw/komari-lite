package main

import (
	"runtime/debug"
	"testing"
)

func TestRuntimeMemoryRespectsExplicitPolicy(t *testing.T) {
	oldGC := debug.SetGCPercent(77)
	oldLimit := debug.SetMemoryLimit(64 << 20)
	defer debug.SetGCPercent(oldGC)
	defer debug.SetMemoryLimit(oldLimit)
	t.Setenv("GOGC", "77")
	t.Setenv("GOMEMLIMIT", "64MiB")
	configureRuntimeMemory()
	if gc := debug.SetGCPercent(77); gc != 77 {
		t.Errorf("explicit GOGC overridden: %d", gc)
	}
	if limit := debug.SetMemoryLimit(-1); limit != 64<<20 {
		t.Errorf("explicit GOMEMLIMIT overridden: %d", limit)
	}
}

func TestRuntimeMemoryRetainsExistingDefaults(t *testing.T) {
	oldGC := debug.SetGCPercent(100)
	oldLimit := debug.SetMemoryLimit(128 << 20)
	defer debug.SetGCPercent(oldGC)
	defer debug.SetMemoryLimit(oldLimit)
	t.Setenv("GOGC", "")
	t.Setenv("GOMEMLIMIT", "")
	configureRuntimeMemory()
	if gc := debug.SetGCPercent(100); gc != 25 {
		t.Errorf("default GC = %d; want existing 25", gc)
	}
	if limit := debug.SetMemoryLimit(-1); limit != 12<<20 {
		t.Errorf("default memory limit = %d; want existing 12MiB", limit)
	}
}
