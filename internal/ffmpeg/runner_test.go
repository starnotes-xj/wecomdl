package ffmpeg

import (
	"slices"
	"strings"
	"testing"

	"wecom-replay-downloader/internal/media"
)

func TestBuildArgsForHLS(t *testing.T) {
	headers := map[string]string{
		"Cookie":     "sid=123",
		"User-Agent": "Mozilla/5.0",
		"Referer":    "https://example.com/page",
		"Host":       "example.com",
		"Range":      "bytes=100-",
		"If-Range":   "etag",
	}
	args := BuildArgs(media.Source{URL: "https://example.com/live.m3u8", Kind: media.KindHLS}, headers, "out.mp4")

	assertContains(t, args, "-user_agent")
	assertContains(t, args, "Mozilla/5.0")
	assertContains(t, args, "-referer")
	assertContains(t, args, "https://example.com/page")
	assertContains(t, args, "-protocol_whitelist")
	assertContains(t, args, "-bsf:a")
	assertContains(t, args, "aac_adtstoasc")
	assertContains(t, args, "out.mp4")
	assertHeaderText(t, args, "Host: example.com\r\n")
	assertHeaderText(t, args, "Cookie: sid=123\r\n")
	assertHeaderText(t, args, "Range: bytes=100-\r\n")
	assertHeaderText(t, args, "If-Range: etag\r\n")
}

func TestBuildArgsSkipsMismatchedHostHeader(t *testing.T) {
	headers := map[string]string{
		"Host":  "wrong.example.com",
		"Range": "bytes=100-",
	}
	args := BuildArgs(media.Source{URL: "https://example.com/live.m3u8", Kind: media.KindHLS}, headers, "out.mp4")

	assertHeaderTextAbsent(t, args, "Host: wrong.example.com\r\n")
	assertHeaderText(t, args, "Range: bytes=100-\r\n")
}

func TestBuildArgsKeepsHostWithoutDefaultPort(t *testing.T) {
	headers := map[string]string{"Host": "example.com"}
	args := BuildArgs(media.Source{URL: "https://example.com:443/live.m3u8", Kind: media.KindHLS}, headers, "out.mp4")

	assertHeaderText(t, args, "Host: example.com\r\n")
}

func TestBuildConcatArgs(t *testing.T) {
	headers := map[string]string{
		"Cookie":     "sid=123",
		"User-Agent": "Mozilla/5.0",
		"Referer":    "https://example.com/page",
		"Host":       "example.com",
	}
	args := BuildConcatArgs("segments.ffconcat", headers, "out.mp4")

	assertContains(t, args, "-protocol_whitelist")
	assertContains(t, args, "file,http,https,tcp,tls,crypto")
	assertContains(t, args, "-safe")
	assertContains(t, args, "0")
	assertContains(t, args, "-f")
	assertContains(t, args, "concat")
	assertContains(t, args, "segments.ffconcat")
	assertContains(t, args, "out.mp4")
	assertContains(t, args, "-user_agent")
	assertContains(t, args, "Mozilla/5.0")
	assertContains(t, args, "-referer")
	assertContains(t, args, "https://example.com/page")
	assertHeaderText(t, args, "Cookie: sid=123\r\n")
	assertHeaderTextAbsent(t, args, "Host: example.com\r\n")
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	if slices.Contains(values, want) {
		return
	}
	t.Fatalf("%q not found in %#v", want, values)
}

func assertNotContains(t *testing.T, values []string, want string) {
	t.Helper()
	if slices.Contains(values, want) {
		t.Fatalf("%q found in %#v", want, values)
	}
}

func assertHeaderText(t *testing.T, args []string, want string) {
	t.Helper()
	for i, arg := range args {
		if arg == "-headers" && i+1 < len(args) && strings.Contains(args[i+1], want) {
			return
		}
	}
	t.Fatalf("header %q not found in %#v", want, args)
}

func assertHeaderTextAbsent(t *testing.T, args []string, want string) {
	t.Helper()
	for i, arg := range args {
		if arg == "-headers" && i+1 < len(args) && strings.Contains(args[i+1], want) {
			t.Fatalf("header %q found in %#v", want, args)
		}
	}
}
