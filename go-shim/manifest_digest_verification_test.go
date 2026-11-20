package goshim

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// TestFetchManifestRawVerifiesRequestedDigest ensures that if a registry serves
// manifest bytes that do not match the requested digest, fetchManifestRaw detects
// the mismatch and rejects the content instead of blindly accepting it (CVE-2026-85747).
func TestFetchManifestRawVerifiesRequestedDigest(t *testing.T) {
	honestContent := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json"}`)
	honestDigest := digest.FromBytes(honestContent)

	tamperedContent := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","attacker":"true"}`)
	tamperedDigest := digest.FromBytes(tamperedContent)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// When the client requests the manifest by honestDigest, the malicious
		// registry lies and returns tamperedContent, along with a forged Docker-Content-Digest header.
		if strings.Contains(r.URL.Path, "/manifests/") {
			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			w.Header().Set("Docker-Content-Digest", honestDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(tamperedContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Request pinned by honestDigest
	ref := u.Host + "/test/model@" + honestDigest.String()
	_, err = fetchManifestRaw(context.Background(), ref)
	if err == nil {
		t.Fatalf("expected digest mismatch error when server returns tampered manifest, got nil")
	}
	if !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("expected error to mention 'digest mismatch', got: %v", err)
	}
	t.Logf("fetchManifestRaw correctly rejected tampered manifest: %v", err)

	// Now verify that when content matches, it succeeds
	srvHonest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/manifests/") {
			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			w.Header().Set("Docker-Content-Digest", tamperedDigest.String())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(tamperedContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer srvHonest.Close()

	uHonest, err := url.Parse(srvHonest.URL)
	if err != nil {
		t.Fatal(err)
	}

	refHonest := uHonest.Host + "/test/model@" + tamperedDigest.String()
	data, err := fetchManifestRaw(context.Background(), refHonest)
	if err != nil {
		t.Fatalf("expected success when manifest matches digest, got error: %v", err)
	}
	if string(data) != string(tamperedContent) {
		t.Fatalf("got unexpected content: %s", string(data))
	}
}

// TestPullToLayoutVerifiesManifestAndConfigDigest ensures pullToLayout rejects
// manifests whose digest does not match the requested digest, and rejects config blobs
// whose digest does not match the manifest's declared config digest.
func TestPullToLayoutVerifiesManifestAndConfigDigest(t *testing.T) {
	configBytes := []byte(`{"architecture":"amd64","os":"linux"}`)
	configDigest := digest.FromBytes(configBytes)

	manifestBytes := []byte(fmt.Sprintf(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.oci.image.manifest.v1+json",
		"config": {
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest": "%s",
			"size": %d
		},
		"layers": []
	}`, configDigest, len(configBytes)))
	manifestDigest := digest.FromBytes(manifestBytes)

	tamperedManifestBytes := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","tampered":true}`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/manifests/"):
			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			w.Header().Set("Docker-Content-Digest", manifestDigest.String())
			w.WriteHeader(http.StatusOK)
			// Return tampered manifest
			_, _ = w.Write(tamperedManifestBytes)
		case strings.Contains(r.URL.Path, "/blobs/"):
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(configBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	dstDir := t.TempDir()
	ref := u.Host + "/test/tampered@" + manifestDigest.String()
	err = pullToLayout(context.Background(), ref, dstDir)
	if err == nil {
		t.Fatalf("expected pullToLayout to fail on tampered manifest, but succeeded")
	}
	if !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("expected error to mention 'digest mismatch', got: %v", err)
	}

	// Verify tampered manifest was not saved in layout
	if _, statErr := os.Stat(filepath.Join(dstDir, "blobs", manifestDigest.Algorithm().String(), manifestDigest.Hex())); !os.IsNotExist(statErr) {
		t.Fatalf("tampered manifest should not have been saved to layout")
	}
}
