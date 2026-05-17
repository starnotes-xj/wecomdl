package media

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIdentifyMediaURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		kind Kind
	}{
		{name: "hls", url: "https://example.com/path/live.m3u8?token=abc", kind: KindHLS},
		{name: "mp4", url: "https://example.com/path/video.mp4?token=abc", kind: KindMP4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := Identify(tt.url)
			if err != nil {
				t.Fatalf("Identify(%q) returned error: %v", tt.url, err)
			}
			if src.Kind != tt.kind {
				t.Fatalf("Identify(%q) kind = %s, want %s", tt.url, src.Kind, tt.kind)
			}
		})
	}
}

func TestIdentifyRejectsNonMedia(t *testing.T) {
	_, err := Identify("https://example.com/live_qrcode?lid=123")
	if err == nil {
		t.Fatal("expected non-media URL error")
	}
}

func TestProbeRejectsHeadHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>login</html>"))
	}))
	defer server.Close()

	_, err := Probe(server.Client(), server.URL+"/live.m3u8", nil)
	if err == nil {
		t.Fatal("expected HTML response to be rejected")
	}
}

func TestProbeOverridesCopiedRangeHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if got := r.Header.Get("Range"); got != "bytes=0-4095" {
			t.Fatalf("Range header = %q, want bytes=0-4095", got)
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n"))
	}))
	defer server.Close()

	_, err := Probe(server.Client(), server.URL+"/live.m3u8", map[string]string{"Range": "bytes=100-"})
	if err != nil {
		t.Fatalf("Probe returned error: %v", err)
	}
}
