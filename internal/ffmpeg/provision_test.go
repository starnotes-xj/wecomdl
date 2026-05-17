package ffmpeg

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureUsesExplicitPath(t *testing.T) {
	exe := writeTempExe(t)
	path, err := Ensure(context.Background(), ProvisionConfig{
		Path:     exe,
		Auto:     true,
		LookPath: missingLookPath,
	}, nil)
	if err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}
	if path != exe {
		t.Fatalf("expected explicit path, got %s", path)
	}
}

func TestEnsureUsesPathLookup(t *testing.T) {
	path, err := Ensure(context.Background(), ProvisionConfig{
		Auto: true,
		LookPath: func(name string) (string, error) {
			return "C:/ffmpeg/bin/ffmpeg.exe", nil
		},
	}, nil)
	if err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}
	if path != "C:/ffmpeg/bin/ffmpeg.exe" {
		t.Fatalf("unexpected path: %s", path)
	}
}

func TestEnsureDownloadsVerifiesAndCaches(t *testing.T) {
	zipData := buildTestZip(t, "pkg/bin/ffmpeg.exe", []byte("fake ffmpeg"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(zipData); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	manifest := testManifest(server.URL, zipData)
	var events []ProvisionEvent
	path, err := Ensure(context.Background(), ProvisionConfig{
		Auto:        true,
		Dir:         t.TempDir(),
		Client:      server.Client(),
		Manifest:    manifest,
		LookPath:    missingLookPath,
		RuntimeOS:   manifest.GOOS,
		RuntimeArch: manifest.GOARCH,
	}, func(event ProvisionEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected extracted ffmpeg: %v", err)
	}
	if !hasProvisionEvent(events, ProvisionDownloadStarted) || !hasProvisionEvent(events, ProvisionVerified) {
		t.Fatalf("missing download events: %#v", events)
	}

	cached, err := Ensure(context.Background(), ProvisionConfig{
		Auto:        true,
		Dir:         filepath.Dir(filepath.Dir(path)),
		Manifest:    manifest,
		LookPath:    missingLookPath,
		RuntimeOS:   manifest.GOOS,
		RuntimeArch: manifest.GOARCH,
	}, nil)
	if err != nil {
		t.Fatalf("cache Ensure returned error: %v", err)
	}
	if cached != path {
		t.Fatalf("expected cached path %s, got %s", path, cached)
	}
}

func TestEnsureRejectsBadChecksum(t *testing.T) {
	zipData := buildTestZip(t, "pkg/bin/ffmpeg.exe", []byte("fake ffmpeg"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(zipData); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	manifest := testManifest(server.URL, zipData)
	manifest.SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	_, err := Ensure(context.Background(), ProvisionConfig{
		Auto:        true,
		Dir:         t.TempDir(),
		Client:      server.Client(),
		Manifest:    manifest,
		LookPath:    missingLookPath,
		RuntimeOS:   manifest.GOOS,
		RuntimeArch: manifest.GOARCH,
	}, nil)
	if err == nil {
		t.Fatal("expected checksum error")
	}
}

func TestEnsureAutoDisabledReturnsInstallHint(t *testing.T) {
	_, err := Ensure(context.Background(), ProvisionConfig{
		Auto:     false,
		Dir:      t.TempDir(),
		LookPath: missingLookPath,
	}, nil)
	if err == nil {
		t.Fatal("expected missing ffmpeg error")
	}
}

func writeTempExe(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg.exe")
	if err := os.WriteFile(path, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func buildTestZip(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	file, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testManifest(url string, data []byte) provisionManifest {
	sum := sha256.Sum256(data)
	return provisionManifest{
		Version:        "test-version",
		GOOS:           "testos",
		GOARCH:         "testarch",
		URL:            url,
		SHA256:         hex.EncodeToString(sum[:]),
		ArchiveExePath: "pkg/bin/ffmpeg.exe",
		MaxBytes:       int64(len(data) + 1024),
	}
}

func missingLookPath(string) (string, error) {
	return "", os.ErrNotExist
}

func hasProvisionEvent(events []ProvisionEvent, kind ProvisionEventKind) bool {
	for _, event := range events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}
