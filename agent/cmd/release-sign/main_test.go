package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/lyhbdw/komari-lite/agent/internal/releasesign"
)

func TestSignReleaseFromPrivateKeyFile(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "agent")
	key := filepath.Join(dir, "test-key.pem")
	out := filepath.Join(dir, "agent.sig.json")
	private := ed25519.NewKeyFromSeed(make([]byte, 32)) // test only
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(key, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(input, []byte("release bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	args := []string{"-private-key", key, "-input", input, "-out", out, "-version", "1.1.8", "-asset", "komari-agent-linux-amd64", "-key-id", "test"}
	if err = run(args); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var manifest releasesign.Manifest
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if err = manifest.Verify(base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey)), "test", "1.1.8", "komari-agent-linux-amd64"); err != nil {
		t.Fatal(err)
	}
	if manifest.SHA256 != "ff7a5e6429d2c8511521e4abf41cd54a3e525ef4a1f24f8d1c67ede9d17874dd" {
		t.Fatalf("wrong input hash %s", manifest.SHA256)
	}
	if err = run([]string{"-private-key", "not-a-private-key-path"}); err == nil {
		t.Fatal("invalid request accepted")
	}
}
