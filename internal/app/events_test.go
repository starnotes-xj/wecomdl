package app

import (
	"context"
	"strings"
	"testing"

	"wecom-replay-downloader/internal/ffmpeg"
	"wecom-replay-downloader/internal/media"
)

func TestRunWithHooksDryRunEmitsRedactedEvents(t *testing.T) {
	var events []Event
	err := RunWithHooks(context.Background(), Options{
		URL:        "https://example.com/video.mp4?token=secret-token&oss_vcode=hidden-code",
		Referer:    "https://live.work.weixin.qq.com/?session=secret-session",
		UserAgent:  "Mozilla/5.0",
		OutputDir:  t.TempDir(),
		OutputName: "video",
		DryRun:     true,
		SkipProbe:  true,
	}, Hooks{
		Eventf: func(event Event) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("RunWithHooks returned error: %v", err)
	}

	if !hasEvent(events, EventRequestBuilt) || !hasEvent(events, EventOutputSelected) || !hasEvent(events, EventDryRunReady) {
		t.Fatalf("missing expected events: %#v", events)
	}
	if hasEvent(events, EventProbeStarted) || hasEvent(events, EventProbeSucceeded) {
		t.Fatalf("skip-probe emitted probe events: %#v", events)
	}

	for _, event := range events {
		if strings.Contains(event.SafeURL, "secret-token") || strings.Contains(event.SafeURL, "hidden-code") {
			t.Fatalf("event leaked sensitive URL: %#v", event)
		}
		if event.Probe != nil && (strings.Contains(event.Probe.SafeURL, "secret-token") || strings.Contains(event.Probe.SafeURL, "hidden-code")) {
			t.Fatalf("probe event leaked sensitive URL: %#v", event.Probe)
		}
		joinedArgs := strings.Join(event.SafeArgs, " ")
		if strings.Contains(joinedArgs, "secret-token") || strings.Contains(joinedArgs, "hidden-code") || strings.Contains(joinedArgs, "secret-session") {
			t.Fatalf("event leaked sensitive args: %#v", event.SafeArgs)
		}
	}
}

func TestProvisionEventToAppEvent(t *testing.T) {
	event := provisionEventToAppEvent(ffmpeg.ProvisionEvent{
		Kind:       ffmpeg.ProvisionDownloadProgress,
		Message:    "正在下载 ffmpeg",
		Path:       "C:/cache/ffmpeg.exe",
		BytesDone:  512,
		BytesTotal: 1024,
	}, "https://example.com/video.mp4", media.KindMP4, "out.mp4")
	if event.Kind != EventFFmpegDownloadProgress || event.FFmpegPath != "C:/cache/ffmpeg.exe" || event.BytesDone != 512 || event.BytesTotal != 1024 {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func hasEvent(events []Event, kind EventKind) bool {
	for _, event := range events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}
