package gui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"wecom-replay-downloader/internal/app"
	"wecom-replay-downloader/internal/redact"
)

type Backend struct {
	ctx     context.Context
	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
}

type DownloadRequest struct {
	Mode       string `json:"mode"`
	URL        string `json:"url"`
	TSListPath string `json:"tsListPath"`
	Referer    string `json:"referer"`
	UserAgent  string `json:"userAgent"`
	OutputDir  string `json:"outputDir"`
	OutputName string `json:"outputName"`
	FFmpegPath string `json:"ffmpegPath"`
	FFmpegAuto bool   `json:"ffmpegAuto"`
	FFmpegDir  string `json:"ffmpegDir"`
	DryRun     bool   `json:"dryRun"`
	SkipProbe  bool   `json:"skipProbe"`
}

type GUIEvent struct {
	Kind        string `json:"kind"`
	Message     string `json:"message"`
	SafeURL     string `json:"safeUrl"`
	SourceKind  string `json:"sourceKind"`
	OutputPath  string `json:"outputPath"`
	FFmpegPath  string `json:"ffmpegPath"`
	BytesDone   int64  `json:"bytesDone"`
	BytesTotal  int64  `json:"bytesTotal"`
	TotalSize   int64  `json:"totalSize"`
	OutTime     string `json:"outTime"`
	Speed       string `json:"speed"`
	Done        bool   `json:"done"`
	StatusCode  int    `json:"statusCode"`
	ContentType string `json:"contentType"`
}

func NewBackend() *Backend {
	return &Backend{}
}

func (b *Backend) Startup(ctx context.Context) {
	b.ctx = ctx
}

func (b *Backend) StartDownload(req DownloadRequest) error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return fmt.Errorf("已有下载任务正在运行")
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.running = true
	b.mu.Unlock()

	go func() {
		defer func() {
			b.mu.Lock()
			b.running = false
			b.cancel = nil
			b.mu.Unlock()
		}()

		err := app.RunWithHooks(ctx, app.Options{
			URL:        req.URL,
			TSListPath: req.TSListPath,
			Referer:    req.Referer,
			UserAgent:  req.UserAgent,
			OutputDir:  req.OutputDir,
			OutputName: req.OutputName,
			FFmpegPath: req.FFmpegPath,
			FFmpegAuto: req.FFmpegAuto,
			FFmpegDir:  req.FFmpegDir,
			DryRun:     req.DryRun,
			SkipProbe:  req.SkipProbe,
		}, app.Hooks{
			Eventf: func(event app.Event) {
				b.emit("download:event", eventToGUI(event))
			},
		})
		if err != nil {
			b.emit("download:error", GUIEvent{Kind: "error", Message: redact.Text(err.Error())})
			return
		}
		b.emit("download:done", GUIEvent{Kind: "done", Message: "任务完成"})
	}()
	return nil
}

func (b *Backend) CancelDownload() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cancel == nil {
		return nil
	}
	b.cancel()
	return nil
}

func (b *Backend) SelectOutputDir() (string, error) {
	return wailsruntime.OpenDirectoryDialog(b.ctx, wailsruntime.OpenDialogOptions{Title: "选择输出目录"})
}

func (b *Backend) SelectTSListFile() (string, error) {
	return wailsruntime.OpenFileDialog(b.ctx, wailsruntime.OpenDialogOptions{
		Title:   "选择 TS 分片列表",
		Filters: []wailsruntime.FileFilter{{DisplayName: "文本文件", Pattern: "*.txt;*.list;*.m3u8"}},
	})
}

func (b *Backend) OpenOutputFile(path string) error {
	if path == "" {
		return nil
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

func (b *Backend) emit(name string, payload GUIEvent) {
	if b.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(b.ctx, name, payload)
}

func eventToGUI(event app.Event) GUIEvent {
	out := GUIEvent{
		Kind:       string(event.Kind),
		Message:    event.Message,
		SafeURL:    event.SafeURL,
		SourceKind: string(event.SourceKind),
		OutputPath: event.OutputPath,
		FFmpegPath: event.FFmpegPath,
		BytesDone:  event.BytesDone,
		BytesTotal: event.BytesTotal,
	}
	if event.Probe != nil {
		out.StatusCode = event.Probe.StatusCode
		out.ContentType = event.Probe.ContentType
	}
	if event.Progress != nil {
		out.TotalSize = event.Progress.TotalSize
		out.OutTime = event.Progress.OutTime.Truncate(time.Second).String()
		out.Speed = event.Progress.Speed
		out.Done = event.Progress.Done
	}
	return out
}
