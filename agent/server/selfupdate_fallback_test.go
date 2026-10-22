package server

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lyhbdw/komari-lite/agent/internal/releasesign"
	v2 "github.com/lyhbdw/komari-lite/agent/protocol/v2"
)

func TestSignedUpdateFallsBackWithoutTrustingMirrorKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	private := ed25519.NewKeyFromSeed(make([]byte, 32))
	oldPub, oldID := TrustedReleasePublicKey, TrustedReleaseKeyID
	TrustedReleasePublicKey = base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	TrustedReleaseKeyID = "test"
	defer func() { TrustedReleasePublicKey, TrustedReleaseKeyID = oldPub, oldID }()
	data := []byte("#!/bin/sh\nexit 0\n")
	manifest := releasesign.Manifest{Version: "9.0.0", Asset: assetName(), SHA256: checksum(string(data)), KeyID: "test"}
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, manifest.Canonical()))
	var binaries atomic.Int32
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".sig.json") {
			switch {
			case strings.HasPrefix(r.URL.Path, "/bad-key"):
				// Even a matching digest with a mirror-provided trust key is invalid.
				json.NewEncoder(w).Encode(map[string]any{"version": manifest.Version, "asset": manifest.Asset, "sha256": manifest.SHA256, "key_id": "mirror", "signature": manifest.Signature, "public_key": TrustedReleasePublicKey})
			case strings.HasPrefix(r.URL.Path, "/missing"):
				w.WriteHeader(404)
			default:
				json.NewEncoder(w).Encode(manifest)
			}
			return
		}
		binaries.Add(1)
		if strings.HasPrefix(r.URL.Path, "/bad-binary") {
			w.Write([]byte("attacker bytes"))
			return
		}
		w.Write(data)
	}))
	defer controller.Close()
	exe := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(exe, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	candidates := []string{controller.URL + "/bad-key", controller.URL + "/missing", controller.URL + "/bad-binary", controller.URL + "/valid"}
	if err := installSignedUpdate(candidates, "9.0.0", assetName(), "", exe); err != nil {
		t.Fatal(err)
	}
	if got := binaries.Load(); got != 2 {
		t.Fatalf("binary requests=%d, expected only authenticated candidates", got)
	}
	got, _ := os.ReadFile(exe)
	if string(got) != string(data) {
		t.Fatal("fallback did not install authentic binary")
	}
}

func TestUpdateWithoutPinnedKeyRejectsWithoutDedup(t *testing.T) {
	resetRawEventState(t)
	oldPub, oldID := TrustedReleasePublicKey, TrustedReleaseKeyID
	TrustedReleasePublicKey, TrustedReleaseKeyID = "", ""
	defer func() { TrustedReleasePublicKey, TrustedReleaseKeyID = oldPub, oldID }()
	if processV2Event(nil, v2.MethodAgentUpdate, map[string]string{"version": "9.0.0"}, "no-trust") {
		t.Fatal("unconfigured update accepted")
	}
	v2AckMu.Lock()
	_, seen := v2SeenEvents["no-trust"]
	v2AckMu.Unlock()
	if seen {
		t.Fatal("unconfigured update marked seen")
	}
}
