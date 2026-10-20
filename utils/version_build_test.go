package utils

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReleaseVersionBuildMetadata(t *testing.T) {
	// Exercise the real linker symbols: -X cannot replace a variable whose
	// initializer calls a function, even though the build itself succeeds.
	directory := t.TempDir()
	source := filepath.Join(directory, "main.go")
	program := `package main
import (
 "fmt"
 "github.com/lyhbdw/komari-lite/utils"
)
func main() { fmt.Printf("%s|%s", utils.CurrentVersion, utils.VersionHash) }
`
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	executable := filepath.Join(directory, "version-probe")
	command := exec.CommandContext(ctx, "go", "build", "-p", "2", "-tags", "nomsgpack,sqlite_omit_load_extension", "-ldflags",
		"-X github.com/lyhbdw/komari-lite/utils.CurrentVersion=7.8.9-test -X github.com/lyhbdw/komari-lite/utils.VersionHash=revision-test", "-o", executable, source)
	command.Env = versionTestEnvironment()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build version probe: %v\n%s", err, output)
	}
	for _, tc := range []struct {
		name string
		env  []string
		want string
	}{
		{name: "compiled metadata", want: "7.8.9-test|revision-test"},
		{name: "explicit runtime override", env: []string{"KOMARI_VERSION=runtime-test", "KOMARI_VERSION_HASH=runtime-hash"}, want: "runtime-test|runtime-hash"},
		{name: "empty overrides preserve build metadata", env: []string{"KOMARI_VERSION=", "KOMARI_VERSION_HASH="}, want: "7.8.9-test|revision-test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probe := exec.CommandContext(ctx, executable)
			probe.Env = append(versionTestEnvironment(), tc.env...)
			output, err := probe.CombinedOutput()
			if err != nil {
				t.Fatalf("run version probe: %v\n%s", err, output)
			}
			if got := strings.TrimSpace(string(output)); got != tc.want {
				t.Fatalf("version identity = %q, want %q", got, tc.want)
			}
		})
	}
}

func versionTestEnvironment() []string {
	var environment []string
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "KOMARI_VERSION=") && !strings.HasPrefix(value, "KOMARI_VERSION_HASH=") {
			environment = append(environment, value)
		}
	}
	return environment
}
