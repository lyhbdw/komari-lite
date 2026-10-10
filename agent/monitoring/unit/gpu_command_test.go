package monitoring

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestGPUCommandHelper(t *testing.T) {
	switch os.Getenv("KOMARI_GPU_HELPER") {
	case "fail":
		os.Exit(7)
	case "hang":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	case "flood":
		_, _ = os.Stdout.WriteString(strings.Repeat("x", 2<<20))
		os.Exit(0)
	}
}

func TestSMIStartPropagatesCommandFailure(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KOMARI_GPU_HELPER", "fail")
	for _, start := range []func() error{(&NvidiaSMI{BinPath: exe}).Start, (&ROCmSMI{BinPath: exe}).Start} {
		if err := start(); err == nil {
			t.Error("SMI.Start swallowed child failure")
		}
	}
}
