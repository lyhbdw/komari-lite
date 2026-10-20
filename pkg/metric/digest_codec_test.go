package metric

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

func TestDigestCodecWorkspaceBudget(t *testing.T) {
	// A fresh process keeps earlier tests and sync.Once initialization from
	// hiding cold-start allocation. TotalAlloc measures cumulative allocation,
	// not retained heap or process RSS.
	const childMarker = "KOMARI_TEST_DIGEST_CODEC_CHILD"
	if os.Getenv(childMarker) != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, executable, "-test.run=^TestDigestCodecWorkspaceBudget$", "-test.count=1", "-test.v")
		command.Env = append(os.Environ(), childMarker+"=1")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated codec allocation check failed: %v\n%s", err, output)
		}
		t.Logf("isolated codec allocation check:\n%s", output)
		return
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	digest := NewTDigest(30)
	for i := 0; i < 120; i++ {
		digest.Add(float64(i%17)+float64(i)/100, 1)
	}
	want := digest.Encode()
	stored := encodeStoredTDigest(digest)
	got, err := decodeStoredTDigest(stored)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("lossless codec round trip failed: %v", err)
	}
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("first-use codec allocation: %d bytes; stored %d / raw %d bytes", allocated, len(stored), len(want))
	if allocated > 512<<10 {
		t.Fatalf("small digest codec allocated %d bytes; budget is 512 KiB", allocated)
	}
}

func TestDigestCodecLegacyAndStoredRemainLossless(t *testing.T) {
	// Match the previous encoder settings rather than testing only the new
	// encoder against its own decoder. Include TU explicitly even when the
	// sample happens to compress well enough to select TZ automatically.
	legacyEncoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(1)))
	if err != nil {
		t.Fatal(err)
	}
	defer legacyEncoder.Close()
	for _, count := range []int{0, 1, 20, 120, 4000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			digest := NewTDigest(30)
			for i := 0; i < count; i++ {
				digest.Add(math.Sin(float64(i))*80+float64(i%97), 1)
			}
			legacy := digest.Encode()
			stored := encodeStoredTDigest(digest)
			rawEnvelope := append([]byte{'T', 'U', storedDigestVersion}, legacy...)
			compressedEnvelope := append([]byte{'T', 'Z', storedDigestVersion}, legacyEncoder.EncodeAll(legacy, nil)...)
			for name, blob := range map[string][]byte{
				"legacy-TD": legacy, "current": stored,
				"raw-TU": rawEnvelope, "previous-encoder-TZ": compressedEnvelope,
			} {
				t.Run(name, func(t *testing.T) {
					decoded, err := DecodeTDigest(blob)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(decoded.Encode(), legacy) {
						t.Fatal("codec changed the digest centroids or metadata")
					}
				})
			}
		})
	}
}

func TestDigestCodecConcurrentRoundTrip(t *testing.T) {
	for i := 0; i < 8; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			digest := NewTDigest(30)
			for j := 0; j < 1000; j++ {
				digest.Add(float64(j%137)+float64(i)/10, 1)
			}
			want := digest.Encode()
			for j := 0; j < 10; j++ {
				got, err := decodeStoredTDigest(encodeStoredTDigest(digest))
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("concurrent codec round trip failed: %v", err)
				}
			}
		})
	}
}
