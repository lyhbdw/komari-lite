package releasesign

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestVerifyBindsReleaseAndPinnedKey(t *testing.T) {
	// Test-only deterministic private key; never used by production.
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	public := base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	sum := sha256.Sum256([]byte("binary"))
	m := Manifest{Version: "1.1.8", Asset: "komari-agent-linux-amd64", SHA256: hex.EncodeToString(sum[:]), KeyID: "test"}
	canonical := "komari-agent-release-v1\n1.1.8\nkomari-agent-linux-amd64\n" + m.SHA256 + "\n"
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, []byte(canonical)))
	if string(m.Canonical()) != canonical {
		t.Fatal("canonical wire format changed")
	}
	if err := m.Verify(public, "test", m.Version, m.Asset); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                        string
		change                      func(*Manifest)
		public, key, version, asset string
	}{
		{name: "unconfigured", public: "", key: "test", version: m.Version, asset: m.Asset},
		{name: "wrong-public", public: base64.StdEncoding.EncodeToString(make([]byte, 32)), key: "test", version: m.Version, asset: m.Asset},
		{name: "unknown-key", public: public, key: "other", version: m.Version, asset: m.Asset},
		{name: "missing-signature", change: func(m *Manifest) { m.Signature = "" }, public: public, key: "test", version: m.Version, asset: m.Asset},
		{name: "tampered-sha", change: func(m *Manifest) { m.SHA256 = strings.Repeat("f", 64) }, public: public, key: "test", version: m.Version, asset: m.Asset},
		{name: "version-replay", public: public, key: "test", version: "1.1.9", asset: m.Asset},
		{name: "asset-replay", public: public, key: "test", version: m.Version, asset: "komari-agent-linux-arm64"},
		{name: "key-id-tamper", change: func(m *Manifest) { m.KeyID = "other" }, public: public, key: "test", version: m.Version, asset: m.Asset},
		{name: "newline-version", change: func(m *Manifest) { m.Version = "1.1.8\nother" }, public: public, key: "test", version: "1.1.8\nother", asset: m.Asset},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := m
			if tc.change != nil {
				tc.change(&copy)
			}
			if err := copy.Verify(tc.public, tc.key, tc.version, tc.asset); err == nil {
				t.Fatal("untrusted manifest accepted")
			}
		})
	}
}
