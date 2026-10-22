package server

// Legacy checksum parser regression only; signed updater never calls this.
import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func fetchSingleSHA256(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := updateHTTPClient(30 * time.Second).Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch sha256: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch sha256: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 513))
	if err != nil {
		return "", fmt.Errorf("fetch sha256: %w", err)
	}
	if len(body) > 512 {
		return "", fmt.Errorf("fetch sha256: response exceeds 512 bytes")
	}
	// 格式：<hex>  <filename>（sha256sum 输出格式）
	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return "", fmt.Errorf("fetch sha256: empty response")
	}
	return validateSHA256(fields[0])
}
