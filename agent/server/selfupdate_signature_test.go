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

	pkgflags "github.com/lyhbdw/komari-lite/agent/cmd/flags"
	"github.com/lyhbdw/komari-lite/agent/internal/releasesign"
)

func TestSignedUpdateAuthenticatesBeforeHelpAndReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell execution fixture")
	}
	private := ed25519.NewKeyFromSeed(make([]byte, 32)) // test-only
	oldPub, oldID := TrustedReleasePublicKey, TrustedReleaseKeyID
	defer func() { TrustedReleasePublicKey, TrustedReleaseKeyID = oldPub, oldID }()
	pub := base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	for _, mode := range []string{"valid", "missing", "wrong-signature", "wrong-version", "wrong-asset", "wrong-key", "wrong-sha", "unconfigured", "event-sha-mismatch", "oversized-manifest"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "executed")
			exe := filepath.Join(dir, "agent")
			original := []byte("original healthy binary")
			if err := os.WriteFile(exe, original, 0755); err != nil {
				t.Fatal(err)
			}
			binary := []byte("#!/bin/sh\nprintf verified > '" + marker + "'\n")
			hash := sha256.Sum256(binary)
			m := releasesign.Manifest{Version: "9.0.0", Asset: assetName(), SHA256: hex.EncodeToString(hash[:]), KeyID: "test"}
			switch mode {
			case "wrong-version":
				m.Version = "9.0.1"
			case "wrong-asset":
				m.Asset = "other"
			case "wrong-key":
				m.KeyID = "mirror"
			case "wrong-sha":
				m.SHA256 = strings.Repeat("0", 64)
			}
			m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, m.Canonical()))
			if mode == "wrong-signature" {
				m.Signature = base64.StdEncoding.EncodeToString(make([]byte, 64))
			}
			TrustedReleasePublicKey, TrustedReleaseKeyID = pub, "test"
			if mode == "unconfigured" {
				TrustedReleasePublicKey = ""
			}
			requests := 0
			controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, ".sig.json") {
					if mode == "missing" {
						w.WriteHeader(404)
						return
					}
					if mode == "oversized-manifest" {
						w.Write([]byte(strings.Repeat(" ", maxReleaseManifestBytes+1)))
						return
					}
					json.NewEncoder(w).Encode(m)
					return
				}
				requests++
				w.Write(binary)
			}))
			defer controller.Close()
			oldFlags := flags
			flags = &pkgflags.Config{}
			defer func() { flags = oldFlags }()
			expected := ""
			if mode == "event-sha-mismatch" {
				expected = strings.Repeat("f", 64)
			}
			// Use local candidate injection only: no external mirrors in negative tests.
			candidates := []string{controller.URL + "/download/agent/9.0.0/" + assetName()}
			err := installSignedUpdate(candidates, "9.0.0", assetName(), expected, exe)
			got, _ := os.ReadFile(exe)
			_, ran := os.Stat(marker)
			if mode == "valid" {
				if err != nil || string(got) != string(binary) || ran != nil {
					t.Fatalf("valid update: err=%v marker=%v binary=%q", err, ran, got)
				}
			} else {
				if err == nil {
					t.Fatal("untrusted update succeeded")
				}
				if ran == nil {
					t.Fatal("untrusted binary executed before authentication")
				}
				if string(got) != string(original) {
					t.Fatal("healthy binary replaced on failure")
				}
				if mode != "wrong-sha" && requests != 0 {
					t.Fatalf("binary requested before manifest authentication: %d", requests)
				}
			}
			stages, _ := filepath.Glob(filepath.Join(dir, ".agent-update-*"))
			if len(stages) != 0 {
				t.Fatalf("staging leaked: %v", stages)
			}
		})
	}
}
