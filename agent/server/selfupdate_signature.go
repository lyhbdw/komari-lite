package server

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/lyhbdw/komari-lite/agent/internal/releasesign"
)

// The release trust anchor ships as a source default so every build verifies
// official releases out of the box: no build flags, no per-release setup. The
// value is a PUBLIC key, which is safe to publish; only the matching private
// key must stay offline. The release builder may still override both pins with
// -ldflags -X (for example to rotate keys ahead of a source release). Neither
// the controller, event params nor mirrors can override these pins.
const (
	defaultTrustedReleasePublicKey = "bDyG4MO5R8KC4fLlC/e2od6O/M9LIvCKi170fDqwtJg="
	defaultTrustedReleaseKeyID     = "komari-release-2026-10"
)

var (
	TrustedReleasePublicKey = defaultTrustedReleasePublicKey
	TrustedReleaseKeyID     = defaultTrustedReleaseKeyID
)

const maxReleaseManifestBytes = 4096

func releaseTrustConfigured() bool {
	pub, err := base64.StdEncoding.DecodeString(TrustedReleasePublicKey)
	return err == nil && len(pub) == ed25519.PublicKeySize && TrustedReleaseKeyID != ""
}

// installSignedUpdate is the only production path to smoke testing/replacing a
// downloaded candidate. Both authentication and the streamed hash precede exec.
func installSignedUpdate(candidates []string, targetVersion, asset, expectedSHA, exePath string) error {
	if !releaseTrustConfigured() {
		return fmt.Errorf("release trust key not configured; refusing update")
	}
	if expectedSHA != "" {
		var err error
		expectedSHA, err = validateSHA256(expectedSHA)
		if err != nil {
			return err
		}
	}
	var lastErr error
	for _, candidate := range candidates {
		manifest, err := fetchReleaseManifest(candidate+".sig.json", targetVersion, asset)
		if err != nil {
			lastErr = err
			continue
		}
		if expectedSHA != "" && expectedSHA != manifest.SHA256 {
			lastErr = fmt.Errorf("event SHA256 differs from signed release")
			continue
		}
		path, err := downloadBinary(candidate, filepath.Dir(exePath), manifest.SHA256)
		if err != nil {
			lastErr = err
			continue
		}
		// The candidate and its manifest may come from any mirror; trust only pins.
		err = smokeTestBinary(path)
		if err == nil {
			err = os.Rename(path, exePath)
		}
		_ = os.Remove(path)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no release download candidates")
	}
	return lastErr
}

func fetchReleaseManifest(url, targetVersion, asset string) (releasesign.Manifest, error) {
	var manifest releasesign.Manifest
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return manifest, err
	}
	resp, err := updateHTTPClient(30 * time.Second).Do(req)
	if err != nil {
		return manifest, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return manifest, fmt.Errorf("release manifest status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReleaseManifestBytes+1))
	if err != nil {
		return manifest, err
	}
	if len(body) > maxReleaseManifestBytes {
		return manifest, fmt.Errorf("release manifest exceeds %d bytes", maxReleaseManifestBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&manifest); err != nil {
		return manifest, err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return manifest, fmt.Errorf("trailing release manifest data")
	}
	return manifest, manifest.Verify(TrustedReleasePublicKey, TrustedReleaseKeyID, targetVersion, asset)
}
