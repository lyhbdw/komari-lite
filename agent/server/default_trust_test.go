package server

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/lyhbdw/komari-lite/agent/internal/releasesign"
)

// A build with no -ldflags -X at all must still trust official releases, so
// auto-update works out of the box instead of silently refusing forever.
func TestDefaultReleaseTrustIsConfiguredWithoutBuildFlags(t *testing.T) {
	pub, err := base64.StdEncoding.DecodeString(defaultTrustedReleasePublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		t.Fatalf("default public key is not a 32-byte Ed25519 key: %v", err)
	}
	if defaultTrustedReleaseKeyID == "" {
		t.Fatal("default key id must not be empty")
	}
	if TrustedReleasePublicKey != defaultTrustedReleasePublicKey || TrustedReleaseKeyID != defaultTrustedReleaseKeyID {
		t.Fatal("package defaults must equal the pinned trust anchor")
	}
	if !releaseTrustConfigured() {
		t.Fatal("a plain build must report release trust as configured")
	}
}

// TestDefaultTrustVerifiesStaticSignedVector proves that the built-in default
// public key genuinely verifies an authentic manifest signed by the release
// private key, without requiring the private key to be present in CI.
func TestDefaultTrustVerifiesStaticSignedVector(t *testing.T) {
	m := releasesign.Manifest{
		Version:   "1.1.8",
		Asset:     "komari-agent-linux-amd64",
		SHA256:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		KeyID:     defaultTrustedReleaseKeyID,
		Signature: "Vu258DnHbIgrAzxBUaN+sLns5o54fcnioxiQEwamG8kz6P2l3O+ZRACgHPYNqb7dq+HMixea4EfA1if3ZRIFDg==",
	}
	if err := m.Verify(defaultTrustedReleasePublicKey, defaultTrustedReleaseKeyID, m.Version, m.Asset); err != nil {
		t.Fatalf("built-in public key failed to verify authentic static vector: %v", err)
	}

	// Tampered version must fail
	if err := m.Verify(defaultTrustedReleasePublicKey, defaultTrustedReleaseKeyID, "1.1.9", m.Asset); err == nil {
		t.Fatal("tampered version unexpectedly passed")
	}
	// Tampered asset must fail
	if err := m.Verify(defaultTrustedReleasePublicKey, defaultTrustedReleaseKeyID, m.Version, "komari-agent-linux-arm64"); err == nil {
		t.Fatal("tampered asset unexpectedly passed")
	}
	// Tampered key ID must fail
	if err := m.Verify(defaultTrustedReleasePublicKey, "other-key", m.Version, m.Asset); err == nil {
		t.Fatal("tampered key ID unexpectedly passed")
	}
}

// loadOperatorKey reads the release private key from the environment so the
// real-signature checks can run without ever putting the key in the repo.
func loadOperatorKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	keyPEM := os.Getenv("KOMARI_RELEASE_PRIVATE_KEY_PEM")
	if keyPEM == "" {
		t.Skip("set KOMARI_RELEASE_PRIVATE_KEY_PEM to run the real-signature check")
	}
	block, rest := pem.Decode([]byte(keyPEM))
	if block == nil || block.Type != "PRIVATE KEY" || len(rest) != 0 {
		t.Fatal("expected one PKCS#8 private key PEM block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse private key: %v", err)
	}
	private, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		t.Fatal("private key file must contain Ed25519")
	}
	return private
}

// The shipped public key must verify a manifest signed by the operator's real
// private key, and reject the same manifest signed by any other key.
func TestDefaultTrustVerifiesRealOperatorSignature(t *testing.T) {
	private := loadOperatorKey(t)
	m := releasesign.Manifest{
		Version: "1.1.9",
		Asset:   "komari-agent-linux-amd64",
		SHA256:  strings.Repeat("a", 64),
		KeyID:   defaultTrustedReleaseKeyID,
	}
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, m.Canonical()))
	if err := m.Verify(defaultTrustedReleasePublicKey, defaultTrustedReleaseKeyID, m.Version, m.Asset); err != nil {
		t.Fatalf("default anchor rejected an operator-signed manifest: %v", err)
	}

	foreign := ed25519.NewKeyFromSeed(make([]byte, 32))
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(foreign, m.Canonical()))
	if err := m.Verify(defaultTrustedReleasePublicKey, defaultTrustedReleaseKeyID, m.Version, m.Asset); err == nil {
		t.Fatal("default anchor accepted a foreign signature")
	}
}

// The manifest the update path fetches must decode and verify with the anchor
// that ships in the source.
func TestDefaultTrustAcceptsRealManifestFromController(t *testing.T) {
	private := loadOperatorKey(t)
	m := releasesign.Manifest{
		Version: "1.1.9",
		Asset:   "komari-agent-linux-amd64",
		SHA256:  strings.Repeat("b", 64),
		KeyID:   defaultTrustedReleaseKeyID,
	}
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, m.Canonical()))
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// installSignedUpdate passes candidate+".sig.json", i.e. the manifest
		// URL for the "<asset>.bin" candidate.
		if !strings.HasSuffix(r.URL.Path, ".bin.sig.json") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(body)
	}))
	defer server.Close()

	got, err := fetchReleaseManifest(server.URL+"/1.1.9/komari-agent-linux-amd64.bin.sig.json", "1.1.9", "komari-agent-linux-amd64")
	if err != nil {
		t.Fatalf("fetchReleaseManifest with the shipped anchor: %v", err)
	}
	if got.SHA256 != m.SHA256 || got.KeyID != m.KeyID {
		t.Fatalf("manifest round-trip mismatch: %+v", got)
	}
}
