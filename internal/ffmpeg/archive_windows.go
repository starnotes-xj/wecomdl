//go:build windows

package ffmpeg

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func extractFFmpegExe(zipPath, archiveExePath, destPath string) error {
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开 ffmpeg 压缩包失败: %w", err)
	}
	defer archive.Close()

	archiveExePath = filepath.ToSlash(archiveExePath)
	for _, file := range archive.File {
		if filepath.ToSlash(file.Name) != archiveExePath {
			continue
		}
		if isUnsafeArchivePath(file.Name) {
			return fmt.Errorf("ffmpeg 压缩包包含不安全路径: %s", file.Name)
		}
		return extractZipFile(file, destPath)
	}
	return fmt.Errorf("ffmpeg 压缩包中未找到 %s", archiveExePath)
}

func extractZipFile(file *zip.File, destPath string) error {
	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("读取 ffmpeg.exe 失败: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("创建 ffmpeg 目录失败: %w", err)
	}
	tmpPath := destPath + ".tmp"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("创建 ffmpeg.exe 临时文件失败: %w", err)
	}
	_, copyErr := io.Copy(out, reader)
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("写入 ffmpeg.exe 失败: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("关闭 ffmpeg.exe 临时文件失败: %w", closeErr)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("保存 ffmpeg.exe 失败: %w", err)
	}
	return nil
}

func isUnsafeArchivePath(path string) bool {
	path = filepath.ToSlash(path)
	return filepath.IsAbs(path) || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "../") || strings.Contains(path, "/../") || strings.Contains(path, ":")
}
