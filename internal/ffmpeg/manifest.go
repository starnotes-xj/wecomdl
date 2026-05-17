package ffmpeg

const defaultFFmpegURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/autobuild-2026-05-10-13-12/ffmpeg-n8.1.1-win64-lgpl-8.1.zip"

type provisionManifest struct {
	Version        string
	GOOS           string
	GOARCH         string
	URL            string
	SHA256         string
	ArchiveExePath string
	MaxBytes       int64
}

var defaultProvisionManifest = provisionManifest{
	Version:        "ffmpeg-n8.1.1-win64-lgpl-8.1",
	GOOS:           "windows",
	GOARCH:         "amd64",
	URL:            defaultFFmpegURL,
	SHA256:         "c6954a85a30dfa297826f1ddf6264188ac63cdd55e51facfbf08bec8ff545858",
	ArchiveExePath: "ffmpeg-n8.1.1-win64-lgpl-8.1/bin/ffmpeg.exe",
	MaxBytes:       200 << 20,
}
