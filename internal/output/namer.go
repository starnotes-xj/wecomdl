package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxBaseNameRunes = 120

var invalidFilenameChars = strings.NewReplacer(
	"<", "_",
	">", "_",
	":", "_",
	"\"", "_",
	"/", "_",
	"\\", "_",
	"|", "_",
	"?", "_",
	"*", "_",
	"\x00", "_",
)

func UniqueMP4Path(dir, name string) (string, error) {
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建输出目录失败: %w", err)
	}

	base := sanitizeName(name)
	if base == "" {
		base = "wecom-replay-" + time.Now().Format("20060102-150405")
	}
	if strings.EqualFold(filepath.Ext(base), ".mp4") {
		base = strings.TrimSuffix(base, filepath.Ext(base))
	}

	for i := 0; i < 10000; i++ {
		suffix := ""
		if i > 0 {
			suffix = fmt.Sprintf(" (%d)", i)
		}

		candidate := filepath.Join(dir, base+suffix+".mp4")
		available, err := availablePath(candidate)
		if err != nil {
			return "", err
		}
		if available {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("无法生成可用输出文件名: %s", base)
}

func availablePath(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return false, nil
	}
	if os.IsNotExist(err) {
		return true, nil
	}
	return false, fmt.Errorf("检查输出文件失败: %w", err)
}

func sanitizeName(name string) string {
	name = invalidFilenameChars.Replace(strings.TrimSpace(name))
	name = strings.Trim(name, " .")

	runes := []rune(name)
	if len(runes) <= maxBaseNameRunes {
		return name
	}
	return string(runes[:maxBaseNameRunes])
}
