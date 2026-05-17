package output

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUniqueMP4PathAutoRename(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "课程回放.mp4")
	if err := os.WriteFile(first, []byte("exists"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := UniqueMP4Path(dir, "课程回放")
	if err != nil {
		t.Fatalf("UniqueMP4Path returned error: %v", err)
	}
	want := filepath.Join(dir, "课程回放 (1).mp4")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUniqueMP4PathSanitizesWindowsName(t *testing.T) {
	dir := t.TempDir()
	got, err := UniqueMP4Path(dir, `清阳易学:八字/曾道篇?`)
	if err != nil {
		t.Fatalf("UniqueMP4Path returned error: %v", err)
	}
	want := filepath.Join(dir, "清阳易学_八字_曾道篇_.mp4")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
