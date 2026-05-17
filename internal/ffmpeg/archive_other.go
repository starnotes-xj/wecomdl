//go:build !windows

package ffmpeg

import "errors"

func extractFFmpegExe(_, _, _ string) error {
	return errors.New("当前平台暂不支持自动解压 Windows ffmpeg")
}
