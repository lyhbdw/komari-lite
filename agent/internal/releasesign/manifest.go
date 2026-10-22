package releasesign

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
)

// Manifest authenticates one release asset. A mirror never supplies trust keys.
type Manifest struct {
	Version   string `json:"version"`
	Asset     string `json:"asset"`
	SHA256    string `json:"sha256"`
	KeyID     string `json:"key_id"`
	Signature string `json:"signature"`
}

func (m Manifest) Canonical() []byte {
	return []byte("komari-agent-release-v1\n" + m.Version + "\n" + m.Asset + "\n" + m.SHA256 + "\n")
}
func (m Manifest) Verify(publicKey, keyID, version, asset string) error {
	pub, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize || keyID == "" {
		return fmt.Errorf("release trust key is not configured or invalid")
	}
	if m.KeyID != keyID {
		return fmt.Errorf("untrusted release key_id")
	}
	if m.Version != version || m.Asset != asset {
		return fmt.Errorf("release version/asset mismatch")
	}
	if !validVersion(m.Version) || m.Asset == "" || path.Base(m.Asset) != m.Asset || strings.ContainsAny(m.Asset, "\\\r\n") {
		return fmt.Errorf("invalid release version/asset")
	}
	sum, err := hex.DecodeString(m.SHA256)
	if err != nil || len(sum) != 32 || strings.ToLower(m.SHA256) != m.SHA256 {
		return fmt.Errorf("invalid canonical sha256")
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize || !ed25519.Verify(ed25519.PublicKey(pub), m.Canonical(), sig) {
		return fmt.Errorf("invalid release signature")
	}
	return nil
}
func validVersion(v string) bool {
	if len(v) == 0 || len(v) > 32 {
		return false
	}
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
