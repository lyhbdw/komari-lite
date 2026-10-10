package boundedexec

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCommandHelper(t *testing.T) {
	switch os.Getenv("KOMARI_COMMAND_HELPER") {
	case "hang":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	case "flood":
		_, _ = os.Stdout.WriteString(strings.Repeat("x", 2<<20))
		os.Exit(0)
	case "ok":
		_, _ = os.Stdout.WriteString("stdout")
		_, _ = os.Stderr.WriteString("stderr")
		os.Exit(0)
	}
}

func TestRunBoundsChildTimeAndOutput(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		mode string
		want error
	}{{"hang", context.DeadlineExceeded}, {"flood", ErrOutputLimit}, {"ok", nil}} {
		t.Run(tc.mode, func(t *testing.T) {
			t.Setenv("KOMARI_COMMAND_HELPER", tc.mode)
			t.Setenv("GORACE", "atexit_sleep_ms=0")
			started := time.Now()
			out, err := Run(context.Background(), 150*time.Millisecond, 1024, exe, "-test.run=^TestCommandHelper$")
			if !errors.Is(err, tc.want) {
				t.Fatalf("Run error = %v, want %v", err, tc.want)
			}
			if len(out) > 1024 {
				t.Fatalf("unbounded output: %d", len(out))
			}
			if time.Since(started) > time.Second {
				t.Fatal("command exceeded time budget")
			}
			if tc.mode == "ok" && (!strings.Contains(string(out), "stdout") || !strings.Contains(string(out), "stderr")) {
				t.Fatalf("output=%q", out)
			}
		})
	}
}
