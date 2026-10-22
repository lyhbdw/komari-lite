package server

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lyhbdw/komari-lite/agent/internal/releasesign"
)

// End-to-end proof that an agent built with NO -ldflags -X at all accepts a
// properly signed release and installs it. This is the "auto-update works out
// of the box" guarantee.
func TestNoFlagBuildAcceptsSignedReleaseEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell execution fixture")
	}
	// The pins must be the shipped source defaults: nothing overrides them.
	if TrustedReleasePublicKey != defaultTrustedReleasePublicKey || TrustedReleaseKeyID != defaultTrustedReleaseKeyID {
		t.Fatal("this test requires the shipped defaults, not injected pins")
	}
	private := loadOperatorKey(t)

	dir := t.TempDir()
	exe := filepath.Join(dir, "agent")
	marker := filepath.Join(dir, "executed")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n[ \"$1\" = \"--help\" ] && touch "+marker+"\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(original)

	m := releasesign.Manifest{
		Version: "1.1.9",
		Asset:   "komari-agent-linux-amd64",
		SHA256:  hex.EncodeToString(sum[:]),
		KeyID:   defaultTrustedReleaseKeyID,
	}
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, m.Canonical()))
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".bin.sig.json"):
			_, _ = w.Write(body)
		case strings.HasSuffix(r.URL.Path, ".bin"):
			_, _ = w.Write(original)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	candidate := server.URL + "/1.1.9/komari-agent-linux-amd64.bin"
	if err := installSignedUpdate([]string{candidate}, "1.1.9", "komari-agent-linux-amd64", "", exe); err != nil {
		t.Fatalf("no-flag build refused a validly signed release: %v", err)
	}
	replaced, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(replaced) != string(original) {
		t.Fatal("binary was not replaced")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("smoke test did not run the replacement: %v", err)
	}
}
