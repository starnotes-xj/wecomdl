package media

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const probeByteLimit = 4096

type Kind string

const (
	KindHLS Kind = "m3u8"
	KindMP4 Kind = "mp4"
)

type Source struct {
	URL  string
	Kind Kind
}

type ProbeResult struct {
	Source      Source
	StatusCode  int
	ContentType string
}

func Identify(rawURL string) (Source, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return Source{}, fmt.Errorf("URL 无效: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Source{}, fmt.Errorf("只支持 http/https URL，当前协议是 %s", parsed.Scheme)
	}

	lowerPath := strings.ToLower(parsed.Path)
	lowerRaw := strings.ToLower(rawURL)
	switch {
	case strings.Contains(lowerPath, ".m3u8") || strings.Contains(lowerRaw, ".m3u8"):
		return Source{URL: rawURL, Kind: KindHLS}, nil
	case strings.Contains(lowerPath, ".mp4") || strings.Contains(lowerRaw, ".mp4"):
		return Source{URL: rawURL, Kind: KindMP4}, nil
	default:
		return Source{}, errors.New("URL 看起来不是 m3u8 或 mp4，请在网络面板搜索 m3u8/mp4 后复制对应请求")
	}
}

func Probe(client *http.Client, rawURL string, headers map[string]string) (ProbeResult, error) {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	src, err := Identify(rawURL)
	if err != nil {
		return ProbeResult{}, err
	}

	result, err := probeOnce(client, http.MethodHead, src, headers)
	if err == nil && result.StatusCode < 400 && result.ContentType != "" {
		if looksHTML(nil, result.ContentType) {
			return result, errors.New("返回内容像 HTML 页面，可能复制到了页面请求或登录页，请复制 m3u8/mp4 媒体请求")
		}
		if isMediaContentType(result.ContentType) {
			result.Source = refineKind(result.Source, result.ContentType, nil)
			return result, nil
		}
	}

	result, body, err := probeRange(client, src, headers)
	if err != nil {
		return ProbeResult{}, err
	}
	result.Source = src
	if result.StatusCode == http.StatusUnauthorized || result.StatusCode == http.StatusForbidden {
		return result, fmt.Errorf("媒体请求返回 %d：会话可能过期，请确认浏览器仍可播放后重新复制视频 URL 或重新捕获", result.StatusCode)
	}
	if result.StatusCode >= 400 {
		return result, fmt.Errorf("媒体请求返回 HTTP %d", result.StatusCode)
	}
	result.Source = refineKind(src, result.ContentType, body)
	if looksHTML(body, result.ContentType) {
		return result, errors.New("返回内容像 HTML 页面，可能复制到了页面请求或登录页，请复制 m3u8/mp4 媒体请求")
	}
	return result, nil
}

func probeOnce(client *http.Client, method string, src Source, headers map[string]string) (ProbeResult, error) {
	req, err := http.NewRequest(method, src.URL, nil)
	if err != nil {
		return ProbeResult{}, err
	}
	applyHeaders(req, headers)
	resp, err := client.Do(req)
	if err != nil {
		return ProbeResult{}, err
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return ProbeResult{}, err
	}
	return ProbeResult{Source: src, StatusCode: resp.StatusCode, ContentType: resp.Header.Get("Content-Type")}, nil
}

func probeRange(client *http.Client, src Source, headers map[string]string) (ProbeResult, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, src.URL, nil)
	if err != nil {
		return ProbeResult{}, nil, err
	}
	applyHeaders(req, headers)
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", probeByteLimit-1))
	resp, err := client.Do(req)
	if err != nil {
		return ProbeResult{}, nil, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, probeByteLimit))
	if readErr != nil {
		return ProbeResult{}, nil, readErr
	}
	return ProbeResult{Source: src, StatusCode: resp.StatusCode, ContentType: resp.Header.Get("Content-Type")}, body, nil
}

func applyHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		if shouldSkipHeader(key) {
			continue
		}
		req.Header.Set(key, value)
	}
}

func shouldSkipHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "host", "content-length", "connection", "accept-encoding", "range", "if-range":
		return true
	default:
		return false
	}
}

func isMediaContentType(contentType string) bool {
	lowerType := strings.ToLower(contentType)
	return strings.Contains(lowerType, "mpegurl") || strings.Contains(lowerType, "vnd.apple.mpegurl") || strings.Contains(lowerType, "video/mp4")
}

func refineKind(src Source, contentType string, body []byte) Source {
	lowerType := strings.ToLower(contentType)
	switch {
	case strings.Contains(lowerType, "mpegurl") || strings.Contains(lowerType, "vnd.apple.mpegurl"):
		src.Kind = KindHLS
	case strings.Contains(lowerType, "video/mp4"):
		src.Kind = KindMP4
	case bytes.HasPrefix(bytes.TrimSpace(body), []byte("#EXTM3U")):
		src.Kind = KindHLS
	case bytes.Contains(body, []byte("ftyp")):
		src.Kind = KindMP4
	}
	return src
}

func looksHTML(body []byte, contentType string) bool {
	lowerType := strings.ToLower(contentType)
	trimmed := bytes.ToLower(bytes.TrimSpace(body))
	return strings.Contains(lowerType, "text/html") || bytes.HasPrefix(trimmed, []byte("<!doctype html")) || bytes.HasPrefix(trimmed, []byte("<html"))
}

// Metadata holds title and date extracted from a URL.
type Metadata struct {
	Title string
	Date  string
}

// ExtractMetadata extracts title and date from query parameters of a media URL.
// It also recursively decodes nested URLs (common in Tencent Cloud CDN URLs).
func ExtractMetadata(rawURL string) Metadata {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return Metadata{}
	}
	var meta Metadata
	extractFromQuery(parsed.Query(), &meta)
	return meta
}

func extractFromQuery(query url.Values, meta *Metadata) {
	if meta.Title == "" {
		meta.Title = query.Get("title")
	}
	if meta.Date == "" {
		for _, key := range []string{"date", "start_time", "play_time", "begin_time"} {
			if v := query.Get(key); v != "" {
				meta.Date = v
				break
			}
		}
	}
	// Recurse into nested URLs (common in Tencent Cloud CDN redirect URLs)
	for _, values := range query {
		for _, value := range values {
			decoded := value
			for i := 0; i < 3; i++ {
				next, err := url.QueryUnescape(decoded)
				if err != nil || next == decoded {
					break
				}
				decoded = next
			}
			if nested, err := url.Parse(decoded); err == nil && nested.RawQuery != "" {
				extractFromQuery(nested.Query(), meta)
			}
		}
	}
}
