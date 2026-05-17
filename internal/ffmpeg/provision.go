package ffmpeg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type ProvisionConfig struct {
	Path        string
	Auto        bool
	Dir         string
	Client      *http.Client
	Manifest    provisionManifest
	LookPath    func(string) (string, error)
	RuntimeOS   string
	RuntimeArch string
}

type ProvisionEventKind string

const (
	ProvisionCheckStarted     ProvisionEventKind = "ffmpeg_check_started"
	ProvisionFound            ProvisionEventKind = "ffmpeg_found"
	ProvisionDownloadStarted  ProvisionEventKind = "ffmpeg_download_started"
	ProvisionDownloadProgress ProvisionEventKind = "ffmpeg_download_progress"
	ProvisionVerified         ProvisionEventKind = "ffmpeg_verified"
)

type ProvisionEvent struct {
	Kind       ProvisionEventKind
	Message    string
	Path       string
	BytesDone  int64
	BytesTotal int64
}

type ProvisionCallback func(ProvisionEvent)

func Ensure(ctx context.Context, config ProvisionConfig, onEvent ProvisionCallback) (string, error) {
	manifest := config.Manifest
	if manifest.Version == "" {
		manifest = defaultProvisionManifest
	}
	emitProvision(onEvent, ProvisionEvent{Kind: ProvisionCheckStarted, Message: "正在检测 ffmpeg"})

	if config.Path != "" {
		path, err := validateExecutable(config.Path)
		if err != nil {
			return "", err
		}
		emitProvision(onEvent, ProvisionEvent{Kind: ProvisionFound, Message: "已使用指定 ffmpeg", Path: path})
		return path, nil
	}

	lookPath := config.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if path, err := lookPath("ffmpeg"); err == nil {
		emitProvision(onEvent, ProvisionEvent{Kind: ProvisionFound, Message: "已在 PATH 中找到 ffmpeg", Path: path})
		return path, nil
	}

	cachePath, err := cacheExePath(config.Dir, manifest)
	if err != nil {
		return "", err
	}
	if executableExists(cachePath) {
		emitProvision(onEvent, ProvisionEvent{Kind: ProvisionFound, Message: "已使用缓存 ffmpeg", Path: cachePath})
		return cachePath, nil
	}

	if !config.Auto {
		return "", errors.New("未找到 ffmpeg，请安装 ffmpeg 并加入 PATH，或使用 --ffmpeg 指定 ffmpeg.exe 路径")
	}
	if err := validateManifest(manifest); err != nil {
		return "", err
	}
	if err := validatePlatform(config, manifest); err != nil {
		return "", err
	}

	archivePath, err := downloadAndVerify(ctx, config, manifest, onEvent)
	if err != nil {
		return "", err
	}
	if err := extractFFmpegExe(archivePath, manifest.ArchiveExePath, cachePath); err != nil {
		return "", err
	}
	emitProvision(onEvent, ProvisionEvent{Kind: ProvisionVerified, Message: "ffmpeg 下载并校验完成", Path: cachePath})
	return cachePath, nil
}

func validateExecutable(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("ffmpeg 路径不可用: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("ffmpeg 路径是目录: %s", path)
	}
	return path, nil
}

func executableExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func validateManifest(manifest provisionManifest) error {
	if manifest.URL == "" {
		return errors.New("ffmpeg 自动安装清单缺少下载地址")
	}
	if manifest.SHA256 == "" {
		return errors.New("ffmpeg 自动安装清单缺少 SHA256")
	}
	if manifest.ArchiveExePath == "" {
		return errors.New("ffmpeg 自动安装清单缺少压缩包内 ffmpeg 路径")
	}
	if manifest.MaxBytes <= 0 {
		return errors.New("ffmpeg 自动安装清单缺少有效大小上限")
	}
	return nil
}

func validatePlatform(config ProvisionConfig, manifest provisionManifest) error {
	goos := config.RuntimeOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := config.RuntimeArch
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	if goos != manifest.GOOS || goarch != manifest.GOARCH {
		return fmt.Errorf("当前平台 %s/%s 暂不支持自动下载 ffmpeg，请手动安装或使用 --ffmpeg 指定路径", goos, goarch)
	}
	return nil
}

func cacheExePath(dir string, manifest provisionManifest) (string, error) {
	if dir == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("获取用户缓存目录失败: %w", err)
		}
		dir = filepath.Join(cacheDir, "wecom-replay-downloader", "ffmpeg")
	}
	return filepath.Join(dir, manifest.Version, "ffmpeg.exe"), nil
}

func downloadAndVerify(ctx context.Context, config ProvisionConfig, manifest provisionManifest, onEvent ProvisionCallback) (string, error) {
	client := config.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifest.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载 ffmpeg 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载 ffmpeg 失败: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > manifest.MaxBytes {
		return "", fmt.Errorf("ffmpeg 下载文件过大: %d bytes", resp.ContentLength)
	}

	root, err := cacheRoot(config.Dir, manifest)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("创建 ffmpeg 缓存目录失败: %w", err)
	}
	archivePath := filepath.Join(root, manifest.Version+".zip")
	tmpPath := archivePath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("创建 ffmpeg 临时文件失败: %w", err)
	}

	emitProvision(onEvent, ProvisionEvent{Kind: ProvisionDownloadStarted, Message: "正在下载 ffmpeg", BytesTotal: resp.ContentLength})
	hash := sha256.New()
	writer := io.MultiWriter(out, hash)
	reader := &limitedProgressReader{reader: resp.Body, max: manifest.MaxBytes, total: resp.ContentLength, onEvent: onEvent}
	_, copyErr := io.Copy(writer, reader)
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("写入 ffmpeg 压缩包失败: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("关闭 ffmpeg 临时文件失败: %w", closeErr)
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if got != manifest.SHA256 {
		os.Remove(tmpPath)
		return "", fmt.Errorf("ffmpeg SHA256 校验失败: got %s", got)
	}
	if err := os.Rename(tmpPath, archivePath); err != nil {
		return "", fmt.Errorf("保存 ffmpeg 压缩包失败: %w", err)
	}
	return archivePath, nil
}

func cacheRoot(dir string, manifest provisionManifest) (string, error) {
	exePath, err := cacheExePath(dir, manifest)
	if err != nil {
		return "", err
	}
	return filepath.Dir(exePath), nil
}

type limitedProgressReader struct {
	reader  io.Reader
	read    int64
	max     int64
	total   int64
	onEvent ProvisionCallback
}

func (r *limitedProgressReader) Read(p []byte) (int, error) {
	if remaining := r.max - r.read; remaining <= 0 {
		var extra [1]byte
		n, err := r.reader.Read(extra[:])
		if n > 0 {
			return 0, fmt.Errorf("ffmpeg 下载文件超过大小限制: %d bytes", r.max)
		}
		return 0, err
	} else if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := r.reader.Read(p)
	if n > 0 {
		r.read += int64(n)
		emitProvision(r.onEvent, ProvisionEvent{Kind: ProvisionDownloadProgress, Message: "正在下载 ffmpeg", BytesDone: r.read, BytesTotal: r.total})
	}
	return n, err
}

func emitProvision(onEvent ProvisionCallback, event ProvisionEvent) {
	if onEvent != nil {
		onEvent(event)
	}
}
