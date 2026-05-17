package redact

import (
	"strings"
	"testing"
)

func TestURLRedactsSensitiveQueryValues(t *testing.T) {
	got := URL("https://example.com/live.m3u8?token=abc&oss_vcode=secret&plain=ok")
	if strings.Contains(got, "abc") || strings.Contains(got, "secret") {
		t.Fatalf("sensitive values were not redacted: %s", got)
	}
	if !strings.Contains(got, "plain=ok") {
		t.Fatalf("non-sensitive query was unexpectedly redacted: %s", got)
	}
}

func TestTextRedactsSensitiveHeadersAndURLs(t *testing.T) {
	got := Text("Cookie: sid=123\nopen https://example.com/a.mp4?token=abc")
	if strings.Contains(got, "sid=123") || strings.Contains(got, "abc") {
		t.Fatalf("sensitive text was not redacted: %s", got)
	}
}

func TestArgsRedactsHeadersInputURLAndReferer(t *testing.T) {
	args := Args([]string{"-headers", "Cookie: sid=123\r\nReferer: https://example.com/page\r\n", "-referer", "https://example.com/?session=secret", "-i", "https://example.com/live.m3u8?token=abc"})
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "sid=123") || strings.Contains(joined, "abc") || strings.Contains(joined, "secret") {
		t.Fatalf("sensitive args were not redacted: %s", joined)
	}
}
