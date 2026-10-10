package cmd

import "testing"

// Lite 版没有子命令机制：曾经的诊断子命令只能以位置参数形式出现，
// 必须被拒绝。
func TestLiteRootExposesOnlyDaemonCommand(t *testing.T) {
	for _, sub := range []string{"check-mem", "list-disk"} {
		if err := parseArgs([]string{sub}); err == nil {
			t.Fatalf("diagnostic subcommand %q must not be accepted in Agent Lite", sub)
		}
	}
}

func TestLiteRootRejectsPositionalArguments(t *testing.T) {
	if err := parseArgs([]string{"some-positional"}); err == nil {
		t.Fatal("positional arguments must be rejected")
	}
}

// 未知 flag 保持静默忽略（与之前 cobra 的 UnknownFlags 白名单行为一致）。
func TestLiteRootIgnoresUnknownFlags(t *testing.T) {
	if err := parseArgs([]string{"--some-future-flag", "value", "--other=1"}); err != nil {
		t.Fatalf("unknown flags must be ignored, got error: %v", err)
	}
}

func TestLiteRootAcceptsKnownFlags(t *testing.T) {
	oldToken, oldEndpoint := flags.Token, flags.Endpoint
	defer func() { flags.Token, flags.Endpoint = oldToken, oldEndpoint }()

	if err := parseArgs([]string{"-t", "tok123", "--endpoint", "https://panel.example"}); err != nil {
		t.Fatalf("known flags must be accepted, got error: %v", err)
	}
	if flags.Token != "tok123" {
		t.Fatalf("expected token %q, got %q", "tok123", flags.Token)
	}
	if flags.Endpoint != "https://panel.example" {
		t.Fatalf("expected endpoint %q, got %q", "https://panel.example", flags.Endpoint)
	}
}
