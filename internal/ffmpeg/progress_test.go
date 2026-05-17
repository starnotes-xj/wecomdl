package ffmpeg

import (
	"testing"
	"time"

	"wecom-replay-downloader/internal/media"
)

func TestProgressParser(t *testing.T) {
	parser := ProgressParser{}
	lines := []string{
		"frame=10",
		"fps=25.5",
		"bitrate=1000.0kbits/s",
		"total_size=123456",
		"out_time_ms=5000000",
		"speed=1.25x",
		"progress=continue",
	}
	var got Progress
	var ok bool
	for _, line := range lines {
		got, ok = parser.ParseLine(line)
	}
	if !ok {
		t.Fatal("expected progress event")
	}
	if got.Frame != 10 || got.FPS != 25.5 || got.TotalSize != 123456 || got.OutTime != 5*time.Second || got.Speed != "1.25x" || got.Done {
		t.Fatalf("unexpected progress: %#v", got)
	}
}

func TestProgressParserEnd(t *testing.T) {
	parser := ProgressParser{}
	got, ok := parser.ParseLine("progress=end")
	if !ok || !got.Done {
		t.Fatalf("expected done progress, got %#v ok=%v", got, ok)
	}
}

func TestBuildArgsWithProgress(t *testing.T) {
	args := BuildArgsWithProgress(media.Source{URL: "https://example.com/live.m3u8", Kind: media.KindHLS}, nil, "out.mp4")
	assertContains(t, args, "-progress")
	assertContains(t, args, "pipe:2")
	assertContains(t, args, "-nostats")
}
