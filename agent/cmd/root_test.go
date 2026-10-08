package cmd

import "testing"

func TestLiteRootExposesOnlyDaemonCommand(t *testing.T) {
	for _, child := range RootCmd.Commands() {
		switch child.Name() {
		case "check-mem", "list-disk":
			t.Fatalf("diagnostic subcommand %q must not be included in Agent Lite", child.Name())
		}
	}
}

func TestLiteRootRejectsPositionalArguments(t *testing.T) {
	if RootCmd.Args == nil {
		t.Fatal("root command must reject positional arguments")
	}
	for _, removedCommand := range []string{"check-mem", "list-disk"} {
		if err := RootCmd.Args(RootCmd, []string{removedCommand}); err == nil {
			t.Fatalf("removed command %q was accepted as a positional argument", removedCommand)
		}
	}
}
