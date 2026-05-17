package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunRequiresURL(t *testing.T) {
	if code := run([]string{"wecomdl"}); code != 2 {
		t.Fatalf("expected exit code 2 without --url, got %d", code)
	}
}

func TestRunURLDryRun(t *testing.T) {
	code := run([]string{
		"wecomdl",
		"--url", "https://example.com/video.mp4",
		"--dry-run",
		"--skip-probe",
		"--out", t.TempDir(),
		"--name", "video",
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunRejectsURLAndTSListTogether(t *testing.T) {
	listPath := filepath.Join(t.TempDir(), "ts.txt")
	if err := os.WriteFile(listPath, []byte("https://example.com/video.ts?start=0&end=99&type=mpegts\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code := run([]string{"wecomdl", "--url", "https://example.com/video.mp4", "--ts-list", listPath})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestRunTSListDryRun(t *testing.T) {
	listPath := filepath.Join(t.TempDir(), "ts.txt")
	if err := os.WriteFile(listPath, []byte("https://example.com/video.ts?start=0&end=99&type=mpegts\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code := run([]string{
		"wecomdl",
		"--ts-list", listPath,
		"--dry-run",
		"--out", t.TempDir(),
		"--name", "video",
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}
