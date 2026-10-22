// release-sign signs a final (post-compression) asset using an operator-owned
// Ed25519 PKCS#8 PEM key file. It never prints or accepts private key bytes.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/lyhbdw/komari-lite/agent/internal/releasesign"
)

func run(args []string) error {
	fs := flag.NewFlagSet("release-sign", flag.ContinueOnError)
	keyFile := fs.String("private-key", "", "Ed25519 PKCS#8 PEM private key FILE")
	input := fs.String("input", "", "final release binary file")
	output := fs.String("out", "", "output manifest JSON file")
	version := fs.String("version", "", "exact target version (v prefix is significant)")
	asset := fs.String("asset", "", "canonical asset name without .bin alias")
	keyID := fs.String("key-id", "", "pinned key identifier")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *keyFile == "" || *input == "" || *output == "" || *version == "" || *asset == "" || *keyID == "" || fs.NArg() != 0 {
		return fmt.Errorf("all six flags are required")
	}
	data, err := os.ReadFile(*keyFile)
	if err != nil {
		return fmt.Errorf("cannot read private key file")
	}
	defer clear(data)
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" || len(rest) != 0 {
		return fmt.Errorf("expected one PKCS#8 private key PEM block")
	}
	defer clear(block.Bytes)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("invalid PKCS#8 key file")
	}
	private, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return fmt.Errorf("private key file must contain Ed25519")
	}
	defer clear(private)
	file, err := os.Open(*input)
	if err != nil {
		return fmt.Errorf("cannot open input binary")
	}
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	file.Close()
	if err != nil {
		return fmt.Errorf("cannot hash input binary")
	}
	m := releasesign.Manifest{Version: *version, Asset: *asset, SHA256: hex.EncodeToString(hash.Sum(nil)), KeyID: *keyID}
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, m.Canonical()))
	pub := base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	if err = m.Verify(pub, *keyID, *version, *asset); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	// Exclusive output avoids accidentally overwriting an input/key file.
	out, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("cannot create output manifest (must not already exist)")
	}
	_, err = out.Write(append(encoded, '\n'))
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		os.Remove(*output)
		return err
	}
	return closeErr
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "release-sign:", err)
		os.Exit(1)
	}
}
