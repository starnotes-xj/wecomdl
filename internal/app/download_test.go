package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRequestFromURL(t *testing.T) {
	req := requestFromURL(Options{
		URL:       "https://example.com/video.mp4",
		Referer:   "https://live.work.weixin.qq.com/",
		UserAgent: "Mozilla/5.0",
	})
	if req.URL != "https://example.com/video.mp4" {
		t.Fatalf("unexpected URL: %s", req.URL)
	}
	if req.Headers["Referer"] != "https://live.work.weixin.qq.com/" {
		t.Fatalf("unexpected referer: %q", req.Headers["Referer"])
	}
	if req.Headers["User-Agent"] != "Mozilla/5.0" {
		t.Fatalf("unexpected user-agent: %q", req.Headers["User-Agent"])
	}
}

func TestPrintDryRunRedactsSensitiveArgs(t *testing.T) {
	output := captureStdout(t, func() {
		printDryRun("ffmpeg", []string{
			"-headers", "Cookie: sid=123\r\nReferer: https://example.com/page\r\n",
			"-i", "https://example.com/live.m3u8?token=abc",
			"out.mp4",
		})
	})
	if strings.Contains(output, "sid=123") || strings.Contains(output, "abc") {
		t.Fatalf("dry-run leaked sensitive values: %s", output)
	}
}

func TestRunWithTSListDryRun(t *testing.T) {
	listPath := filepath.Join(t.TempDir(), "ts.txt")
	if err := os.WriteFile(listPath, []byte("https://example.com/video.ts?start=0&end=99&type=mpegts\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var events []Event
	err := RunWithHooks(context.Background(), Options{
		TSListPath: listPath,
		Referer:    "https://live.work.weixin.qq.com/",
		UserAgent:  "Mozilla/5.0",
		OutputDir:  t.TempDir(),
		OutputName: "video",
		DryRun:     true,
	}, Hooks{
		Eventf: func(event Event) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("RunWithHooks returned error: %v", err)
	}
	if !downloadTestHasEvent(events, EventRequestBuilt) || !downloadTestHasEvent(events, EventOutputSelected) || !downloadTestHasEvent(events, EventDryRunReady) {
		t.Fatalf("missing expected events: %#v", events)
	}
	var args []string
	for _, event := range events {
		if event.Kind == EventDryRunReady {
			args = event.SafeArgs
		}
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-f", "concat", "-safe", "0", "-protocol_whitelist"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected dry-run args to contain %q: %#v", want, args)
		}
	}
}

func TestEscapeConcatURL(t *testing.T) {
	got := escapeConcatURL(`https://example.com/a'b\\c.ts?start=0&end=99&type=mpegts`)
	want := `https://example.com/a'\''b\\\\c.ts?start=0&end=99&type=mpegts`
	if got != want {
		t.Fatalf("escapeConcatURL() = %q, want %q", got, want)
	}
}

func downloadTestHasEvent(events []Event, kind EventKind) bool {
	for _, event := range events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
