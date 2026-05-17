package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"wecom-replay-downloader/internal/app"
)

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	programName := filepath.Base(args[0])
	fs := flag.NewFlagSet(programName, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	const defaultReferer = "https://live.work.weixin.qq.com/"

	url := fs.String("url", "", "直接传入 m3u8/mp4 视频地址，工具会自动构造请求")
	tsList := fs.String("ts-list", "", "TS 分片 URL 列表文件，每行一个分片；仅在没有 m3u8/mp4 时作为兜底")
	referer := fs.String("referer", defaultReferer, "--url 模式使用的 Referer")
	userAgent := fs.String("user-agent", defaultUserAgent, "--url 模式使用的 User-Agent")
	outDir := fs.String("out", ".", "输出目录")
	name := fs.String("name", "", "输出文件名，不需要 .mp4；为空时自动生成")
	ffmpegPath := fs.String("ffmpeg", "", "ffmpeg.exe 路径；为空时自动检测 PATH，必要时下载固定版本")
	ffmpegAuto := fs.Bool("ffmpeg-auto", true, "未找到 ffmpeg 时自动下载固定版本并校验")
	ffmpegDir := fs.String("ffmpeg-dir", "", "自动下载 ffmpeg 的缓存目录；为空时使用用户缓存目录")
	dryRun := fs.Bool("dry-run", false, "只打印 ffmpeg 参数，不实际下载")
	skipProbe := fs.Bool("skip-probe", false, "跳过下载前媒体探测")
	timeout := fs.Duration("timeout", 0, "整体超时，例如 30m；0 表示不限制")

	if err := fs.Parse(args[1:]); err != nil {
		printUsage(programName)
		return 2
	}
	if (*url == "") == (*tsList == "") {
		printUsage(programName)
		return 2
	}

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	opts := app.Options{
		URL:        *url,
		TSListPath: *tsList,
		Referer:    *referer,
		UserAgent:  *userAgent,
		OutputDir:  *outDir,
		OutputName: *name,
		FFmpegPath: *ffmpegPath,
		FFmpegAuto: *ffmpegAuto,
		FFmpegDir:  *ffmpegDir,
		DryRun:     *dryRun,
		SkipProbe:  *skipProbe,
	}

	if err := app.Run(ctx, opts); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	return 0
}

func printUsage(programName string) {
	fmt.Fprintf(os.Stderr, `用法:
  %[1]s --url "https://example/video.mp4" --out D:\Videos --name "课程回放"
  %[1]s --ts-list ts.txt --out D:\Videos --name "课程回放-ts"

操作步骤:
  1. 优先从响应 JSON 里复制 trans_video_url 的 mp4，或 video_url 的 m3u8。
  2. 使用 --url 直接下载；默认会带企业微信直播 Referer 和常见 User-Agent。
  3. 只有抓不到 mp4/m3u8、只能拿到多个 .ts 分片时，才把分片 URL 每行一个保存到文本文件并使用 --ts-list。

ffmpeg:
  默认会从 PATH 检测 ffmpeg；找不到时自动下载固定版本并校验 SHA256。
  如需关闭自动下载，可加 --ffmpeg-auto=false；如已安装，可用 --ffmpeg 指定路径。

参数:
`, programName)
}
