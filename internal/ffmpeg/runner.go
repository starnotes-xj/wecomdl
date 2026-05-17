package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"

	"wecom-replay-downloader/internal/media"
	"wecom-replay-downloader/internal/redact"
)

type Runner struct {
	Path string
}

func (r Runner) Run(ctx context.Context, src media.Source, headers map[string]string, outputPath string) error {
	return r.run(ctx, BuildArgs(src, headers, outputPath), nil)
}

func (r Runner) RunWithProgress(ctx context.Context, src media.Source, headers map[string]string, outputPath string, onProgress func(Progress)) error {
	return r.run(ctx, BuildArgsWithProgress(src, headers, outputPath), onProgress)
}

func (r Runner) RunConcat(ctx context.Context, manifestPath string, headers map[string]string, outputPath string) error {
	return r.run(ctx, BuildConcatArgs(manifestPath, headers, outputPath), nil)
}

func (r Runner) RunConcatWithProgress(ctx context.Context, manifestPath string, headers map[string]string, outputPath string, onProgress func(Progress)) error {
	return r.run(ctx, BuildConcatArgsWithProgress(manifestPath, headers, outputPath), onProgress)
}

func (r Runner) run(ctx context.Context, args []string, onProgress func(Progress)) error {
	path := r.Path
	if path == "" {
		var err error
		path, err = exec.LookPath("ffmpeg")
		if err != nil {
			return errors.New("未找到 ffmpeg，请安装 ffmpeg 并加入 PATH，或使用 --ffmpeg 指定 ffmpeg.exe 路径")
		}
	}

	if onProgress != nil {
		args = progressArgs(args)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout = os.Stdout

	if onProgress == nil {
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return classifyError(err, stderr.String())
		}
		return nil
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	stderrText := readProgress(stderrPipe, onProgress)
	if err := cmd.Wait(); err != nil {
		return classifyError(err, stderrText)
	}
	return nil
}

func BuildArgs(src media.Source, headers map[string]string, outputPath string) []string {
	args := []string{"-hide_banner", "-y"}
	args = append(args, commonArgs(src, headers, outputPath)...)
	return args
}

func BuildArgsWithProgress(src media.Source, headers map[string]string, outputPath string) []string {
	return progressArgs(BuildArgs(src, headers, outputPath))
}

func BuildConcatArgs(manifestPath string, headers map[string]string, outputPath string) []string {
	args := []string{"-hide_banner", "-y", "-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "10"}
	appendInputHeaders(&args, media.Source{URL: "", Kind: media.KindHLS}, headers)
	args = append(args, "-protocol_whitelist", "file,http,https,tcp,tls,crypto", "-safe", "0", "-f", "concat", "-i", manifestPath, "-c", "copy", outputPath)
	return args
}

func BuildConcatArgsWithProgress(manifestPath string, headers map[string]string, outputPath string) []string {
	return progressArgs(BuildConcatArgs(manifestPath, headers, outputPath))
}

func progressArgs(args []string) []string {
	out := []string{"-hide_banner", "-y", "-nostats", "-progress", "pipe:2"}
	return append(out, args[2:]...)
}

func commonArgs(src media.Source, headers map[string]string, outputPath string) []string {
	args := []string{"-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "10"}

	appendInputHeaders(&args, src, headers)
	if src.Kind == media.KindHLS {
		args = append(args, "-protocol_whitelist", "http,https,tcp,tls,crypto")
	}

	args = append(args, "-i", src.URL, "-c", "copy")
	if src.Kind == media.KindHLS {
		args = append(args, "-bsf:a", "aac_adtstoasc")
	}
	args = append(args, outputPath)
	return args
}

func appendInputHeaders(args *[]string, src media.Source, headers map[string]string) {
	if ua := headerValue(headers, "User-Agent"); ua != "" {
		*args = append(*args, "-user_agent", ua)
	}
	if referer := headerValue(headers, "Referer"); referer != "" {
		*args = append(*args, "-referer", referer)
	}
	if headerText := ffmpegHeaders(src, headers); headerText != "" {
		*args = append(*args, "-headers", headerText)
	}
}

func readProgress(stderr io.Reader, onProgress func(Progress)) string {
	var stderrText strings.Builder
	parser := ProgressParser{}
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		stderrText.WriteString(line)
		stderrText.WriteByte('\n')
		if progress, ok := parser.ParseLine(line); ok {
			onProgress(progress)
		}
	}
	return stderrText.String()
}

func ffmpegHeaders(src media.Source, headers map[string]string) string {
	var b strings.Builder
	for _, key := range sortedHeaderKeys(headers) {
		value := headers[key]
		if shouldSkipHeader(src, key, value) {
			continue
		}
		fmt.Fprintf(&b, "%s: %s\r\n", key, value)
	}
	return b.String()
}

func sortedHeaderKeys(headers map[string]string) []string {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func shouldSkipHeader(src media.Source, key, value string) bool {
	if value == "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "content-length", "proxy-authenticate", "proxy-authorization":
		return true
	case "host":
		return src.URL == "" || !hostMatchesSource(value, src.URL)
	default:
		return false
	}
}

func hostMatchesSource(headerHost, rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return false
	}
	headerHost = strings.TrimSpace(headerHost)
	if strings.EqualFold(headerHost, parsed.Host) {
		return true
	}
	if _, _, err := net.SplitHostPort(headerHost); err == nil {
		return false
	}
	return strings.EqualFold(headerHost, parsed.Hostname())
}

func headerValue(headers map[string]string, name string) string {
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}

func classifyError(err error, stderr string) error {
	redactedStderr := redact.Text(tail(stderr, 2000))
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "401") || strings.Contains(lower, "403") || strings.Contains(lower, "forbidden") || strings.Contains(lower, "unauthorized"):
		return fmt.Errorf("ffmpeg 下载失败：会话可能过期或无权限，请确认浏览器仍可播放后重新复制视频 URL 或重新捕获\n%s", redactedStderr)
	case strings.Contains(lower, "drm") || strings.Contains(lower, "widevine") || strings.Contains(lower, "fairplay") || strings.Contains(lower, "playready"):
		return fmt.Errorf("ffmpeg 下载失败：疑似 DRM 保护内容，本工具不会绕过 DRM\n%s", redactedStderr)
	default:
		return fmt.Errorf("ffmpeg 执行失败: %w\n%s", err, redactedStderr)
	}
}

func tail(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[len(s)-limit:]
}
