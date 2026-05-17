package app

import (
	"net/http"

	"wecom-replay-downloader/internal/ffmpeg"
	"wecom-replay-downloader/internal/media"
)

type EventKind string

const (
	EventRequestBuilt           EventKind = "request_built"
	EventProbeStarted           EventKind = "probe_started"
	EventProbeSucceeded         EventKind = "probe_succeeded"
	EventOutputSelected         EventKind = "output_selected"
	EventDryRunReady            EventKind = "dry_run_ready"
	EventFFmpegCheckStarted     EventKind = "ffmpeg_check_started"
	EventFFmpegFound            EventKind = "ffmpeg_found"
	EventFFmpegDownloadStarted  EventKind = "ffmpeg_download_started"
	EventFFmpegDownloadProgress EventKind = "ffmpeg_download_progress"
	EventFFmpegVerified         EventKind = "ffmpeg_verified"
	EventDownloadStarted        EventKind = "download_started"
	EventDownloadProgress       EventKind = "download_progress"
	EventDownloadFinished       EventKind = "download_finished"
)

type ProbeSummary struct {
	StatusCode  int
	ContentType string
	SourceKind  media.Kind
	SafeURL     string
}

type Event struct {
	Kind       EventKind
	Message    string
	SafeURL    string
	SourceKind media.Kind
	OutputPath string
	FFmpegPath string
	BytesDone  int64
	BytesTotal int64
	Probe      *ProbeSummary
	Progress   *ffmpeg.Progress
	SafeArgs   []string
}

type Hooks struct {
	Logf   func(format string, args ...any)
	Eventf func(Event)
	Runner ffmpeg.Runner
	Client *http.Client
}

func (h Hooks) emit(event Event) {
	if h.Eventf != nil {
		h.Eventf(event)
	}
}

func (h Hooks) logf(format string, args ...any) {
	if h.Logf != nil {
		h.Logf(format, args...)
	}
}
