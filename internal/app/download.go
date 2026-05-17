package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wecom-replay-downloader/internal/ffmpeg"
	"wecom-replay-downloader/internal/media"
	"wecom-replay-downloader/internal/output"
	"wecom-replay-downloader/internal/redact"
)

type Options struct {
	URL        string
	TSListPath string
	Referer    string
	UserAgent  string
	OutputDir  string
	OutputName string
	FFmpegPath string
	FFmpegAuto bool
	FFmpegDir  string
	DryRun     bool
	SkipProbe  bool
}

type mediaRequest struct {
	URL     string
	Headers map[string]string
}

func Run(ctx context.Context, opts Options) error {
	return RunWithHooks(ctx, opts, Hooks{
		Logf: func(format string, args ...any) {
			fmt.Printf(format, args...)
		},
	})
}

func RunWithHooks(ctx context.Context, opts Options, hooks Hooks) error {
	if opts.URL != "" {
		meta := media.ExtractMetadata(opts.URL)
		return runRequestDownload(ctx, opts, requestFromURL(opts), metaToOutputName(meta), hooks)
	}
	if opts.TSListPath != "" {
		return runTSMerge(ctx, opts, hooks)
	}
	return fmt.Errorf("请使用 --url 指定视频地址，或使用 --ts-list 指定 TS 分片列表")
}

func runTSMerge(ctx context.Context, opts Options, hooks Hooks) error {
	data, err := os.ReadFile(opts.TSListPath)
	if err != nil {
		return err
	}
	segments, err := media.ParseTSList(data)
	if err != nil {
		return err
	}
	headers := headersFromOptions(opts)
	safeURL := fmt.Sprintf("%d 个 TS 分片", len(segments))
	hooks.emit(Event{Kind: EventRequestBuilt, Message: "已解析 TS 分片列表", SafeURL: safeURL})

	outPath, err := output.UniqueMP4Path(opts.OutputDir, opts.OutputName)
	if err != nil {
		return err
	}
	hooks.emit(Event{Kind: EventOutputSelected, Message: "已选择输出文件", SafeURL: safeURL, OutputPath: outPath})

	manifestPath, cleanup, err := writeConcatManifest(segments)
	if err != nil {
		return err
	}
	defer cleanup()

	args := ffmpeg.BuildConcatArgs(manifestPath, headers, outPath)
	if opts.DryRun {
		safeArgs := redact.Args(args)
		hooks.emit(Event{Kind: EventDryRunReady, Message: "dry-run 参数已生成", SafeURL: safeURL, OutputPath: outPath, SafeArgs: safeArgs})
		printDryRunWithLog(hooks.Logf, opts.FFmpegPath, safeArgs)
		return nil
	}

	runner := hooks.Runner
	if runner.Path == "" {
		ffmpegPath, err := ffmpeg.Ensure(ctx, ffmpeg.ProvisionConfig{
			Path: opts.FFmpegPath,
			Auto: opts.FFmpegAuto,
			Dir:  opts.FFmpegDir,
		}, func(event ffmpeg.ProvisionEvent) {
			hooks.emit(provisionEventToAppEvent(event, safeURL, "", outPath))
		})
		if err != nil {
			return err
		}
		runner.Path = ffmpegPath
	}

	hooks.logf("开始合并 TS 分片: %d 个\n", len(segments))
	hooks.logf("输出文件: %s\n", outPath)
	hooks.emit(Event{Kind: EventDownloadStarted, Message: "开始合并 TS 分片", SafeURL: safeURL, OutputPath: outPath, FFmpegPath: runner.Path})
	if hooks.Eventf != nil {
		err = runner.RunConcatWithProgress(ctx, manifestPath, headers, outPath, func(progress ffmpeg.Progress) {
			hooks.emit(Event{Kind: EventDownloadProgress, Message: "合并中", SafeURL: safeURL, OutputPath: outPath, Progress: &progress})
		})
	} else {
		err = runner.RunConcat(ctx, manifestPath, headers, outPath)
	}
	if err != nil {
		return err
	}
	hooks.logf("合并完成: %s\n", outPath)
	hooks.emit(Event{Kind: EventDownloadFinished, Message: "合并完成", SafeURL: safeURL, OutputPath: outPath})
	return nil
}

func writeConcatManifest(segments []media.Segment) (string, func(), error) {
	file, err := os.CreateTemp("", "wecomdl-*.ffconcat")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.Remove(file.Name()) }
	if _, err := file.WriteString("ffconcat version 1.0\n"); err != nil {
		file.Close()
		cleanup()
		return "", func() {}, err
	}
	for _, segment := range segments {
		if _, err := fmt.Fprintf(file, "file '%s'\n", escapeConcatURL(segment.URL)); err != nil {
			file.Close()
			cleanup()
			return "", func() {}, err
		}
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return filepath.ToSlash(file.Name()), cleanup, nil
}

func escapeConcatURL(raw string) string {
	raw = strings.ReplaceAll(raw, `\`, `\\`)
	return strings.ReplaceAll(raw, "'", "'\\''")
}

func runRequestDownload(ctx context.Context, opts Options, req mediaRequest, outputName string, hooks Hooks) error {
	if opts.OutputName == "" && outputName != "" {
		opts.OutputName = outputName
	}
	safeURL := redact.URL(req.URL)
	hooks.emit(Event{Kind: EventRequestBuilt, Message: "已解析输入", SafeURL: safeURL})

	src, err := media.Identify(req.URL)
	if err != nil {
		return err
	}

	if !opts.SkipProbe {
		client := hooks.Client
		if client == nil {
			client = &http.Client{Timeout: 20 * time.Second}
		}
		hooks.emit(Event{Kind: EventProbeStarted, Message: "正在探测媒体", SafeURL: safeURL, SourceKind: src.Kind})
		probe, err := media.Probe(client, req.URL, req.Headers)
		if err != nil {
			return err
		}
		src = probe.Source
		hooks.logf("探测成功: HTTP %d, Content-Type: %s, 类型: %s\n", probe.StatusCode, probe.ContentType, src.Kind)
		hooks.emit(Event{
			Kind:       EventProbeSucceeded,
			Message:    "媒体探测成功",
			SafeURL:    safeURL,
			SourceKind: src.Kind,
			Probe: &ProbeSummary{
				StatusCode:  probe.StatusCode,
				ContentType: probe.ContentType,
				SourceKind:  src.Kind,
				SafeURL:     safeURL,
			},
		})
	}

	outPath, err := output.UniqueMP4Path(opts.OutputDir, opts.OutputName)
	if err != nil {
		return err
	}
	hooks.emit(Event{Kind: EventOutputSelected, Message: "已选择输出文件", SafeURL: safeURL, SourceKind: src.Kind, OutputPath: outPath})

	args := ffmpeg.BuildArgs(src, req.Headers, outPath)
	if opts.DryRun {
		safeArgs := redact.Args(args)
		hooks.emit(Event{Kind: EventDryRunReady, Message: "dry-run 参数已生成", SafeURL: safeURL, SourceKind: src.Kind, OutputPath: outPath, SafeArgs: safeArgs})
		printDryRunWithLog(hooks.Logf, opts.FFmpegPath, safeArgs)
		return nil
	}

	runner := hooks.Runner
	if runner.Path == "" {
		ffmpegPath, err := ffmpeg.Ensure(ctx, ffmpeg.ProvisionConfig{
			Path: opts.FFmpegPath,
			Auto: opts.FFmpegAuto,
			Dir:  opts.FFmpegDir,
		}, func(event ffmpeg.ProvisionEvent) {
			hooks.emit(provisionEventToAppEvent(event, safeURL, src.Kind, outPath))
		})
		if err != nil {
			return err
		}
		runner.Path = ffmpegPath
	}

	hooks.logf("开始下载: %s\n", safeURL)
	hooks.logf("输出文件: %s\n", outPath)
	hooks.emit(Event{Kind: EventDownloadStarted, Message: "开始下载", SafeURL: safeURL, SourceKind: src.Kind, OutputPath: outPath, FFmpegPath: runner.Path})
	if hooks.Eventf != nil {
		err = runner.RunWithProgress(ctx, src, req.Headers, outPath, func(progress ffmpeg.Progress) {
			hooks.emit(Event{Kind: EventDownloadProgress, Message: "下载中", SafeURL: safeURL, SourceKind: src.Kind, OutputPath: outPath, Progress: &progress})
		})
	} else {
		err = runner.Run(ctx, src, req.Headers, outPath)
	}
	if err != nil {
		return err
	}
	hooks.logf("下载完成: %s\n", outPath)
	hooks.emit(Event{Kind: EventDownloadFinished, Message: "下载完成", SafeURL: safeURL, SourceKind: src.Kind, OutputPath: outPath})
	return nil
}

func provisionEventToAppEvent(event ffmpeg.ProvisionEvent, safeURL string, sourceKind media.Kind, outputPath string) Event {
	return Event{
		Kind:       provisionEventKind(event.Kind),
		Message:    event.Message,
		SafeURL:    safeURL,
		SourceKind: sourceKind,
		OutputPath: outputPath,
		FFmpegPath: event.Path,
		BytesDone:  event.BytesDone,
		BytesTotal: event.BytesTotal,
	}
}

func provisionEventKind(kind ffmpeg.ProvisionEventKind) EventKind {
	switch kind {
	case ffmpeg.ProvisionFound:
		return EventFFmpegFound
	case ffmpeg.ProvisionDownloadStarted:
		return EventFFmpegDownloadStarted
	case ffmpeg.ProvisionDownloadProgress:
		return EventFFmpegDownloadProgress
	case ffmpeg.ProvisionVerified:
		return EventFFmpegVerified
	default:
		return EventFFmpegCheckStarted
	}
}

func requestFromURL(opts Options) mediaRequest {
	return mediaRequest{URL: opts.URL, Headers: headersFromOptions(opts)}
}

func headersFromOptions(opts Options) map[string]string {
	headers := map[string]string{}
	if opts.UserAgent != "" {
		headers["User-Agent"] = opts.UserAgent
	}
	if opts.Referer != "" {
		headers["Referer"] = opts.Referer
	}
	return headers
}

func metaToOutputName(meta media.Metadata) string {
	var parts []string
	if meta.Date != "" {
		if normalized := normalizeDate(meta.Date); normalized != "" {
			parts = append(parts, normalized)
		}
	}
	if meta.Title != "" {
		parts = append(parts, meta.Title)
	}
	return strings.Join(parts, " ")
}

func normalizeDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if ts, err := strconv.ParseInt(raw, 10, 64); err == nil && ts > 0 {
		t := time.Unix(ts, 0)
		return t.Format("060102")
	}
	if len(raw) == 8 {
		if t, err := time.Parse("20060102", raw); err == nil {
			return t.Format("060102")
		}
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.Format("060102")
	}
	if t, err := time.Parse("2006/01/02", raw); err == nil {
		return t.Format("060102")
	}
	if len(raw) == 6 {
		if _, err := time.Parse("060102", raw); err == nil {
			return raw
		}
	}
	return ""
}

func printDryRunWithLog(logf func(format string, args ...any), ffmpegPath string, args []string) {
	if logf == nil {
		return
	}
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	logf("ffmpeg 参数:\n")
	logf("%s", ffmpegPath)
	for _, arg := range args {
		logf(" %q", arg)
	}
	logf("\n")
}

func printDryRun(ffmpegPath string, args []string) {
	printDryRunWithLog(func(format string, args ...any) {
		fmt.Printf(format, args...)
	}, ffmpegPath, redact.Args(args))
}
