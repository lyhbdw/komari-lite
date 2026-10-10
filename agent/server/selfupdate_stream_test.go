package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type repeatedByteReader struct{}

func (repeatedByteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

func TestDownloadRejectsOversizedChunkedResponse(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		_, _ = io.Copy(w, io.LimitReader(repeatedByteReader{}, (100<<20)+1))
	}))
	defer s.Close()
	_, err := downloadBinary(s.URL, t.TempDir(), strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("oversized download was silently truncated and accepted")
	}
}

func TestFetchSHA256RejectsWrongDigestLength(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "abcd  asset\n") }))
	defer s.Close()
	if _, err := fetchSingleSHA256(s.URL); err == nil {
		t.Fatal("short SHA256 digest was accepted")
	}
}

func TestFetchSHA256RejectsOversizedResponse(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("a", 64)+" "+strings.Repeat("x", 1024))
	}))
	defer s.Close()
	if _, err := fetchSingleSHA256(s.URL); err == nil {
		t.Fatal("truncated checksum response was accepted")
	}
}
