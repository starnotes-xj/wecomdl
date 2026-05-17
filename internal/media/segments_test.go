package media

import "testing"

func TestParseTSListSortsByStart(t *testing.T) {
	segments, err := ParseTSList([]byte(`
# comment
https://1253731777.vod2.myqcloud.com/path/video.ts?start=200&end=299&type=mpegts&resolution=2080x1168
https://1253731777.vod2.myqcloud.com/path/video.ts?start=0&end=99&type=mpegts&resolution=2080x1168
https://1253731777.vod2.myqcloud.com/path/video.ts?start=100&end=199&type=mpegts&resolution=2080x1168
`))
	if err != nil {
		t.Fatalf("ParseTSList returned error: %v", err)
	}
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segments))
	}
	for i, want := range []int64{0, 100, 200} {
		if segments[i].Start != want {
			t.Fatalf("segment %d start = %d, want %d", i, segments[i].Start, want)
		}
	}
}

func TestParseTSListDeduplicatesURLs(t *testing.T) {
	raw := "https://example.com/video.ts?start=0&end=99&type=mpegts"
	segments, err := ParseTSList([]byte(raw + "\n" + raw + "\n"))
	if err != nil {
		t.Fatalf("ParseTSList returned error: %v", err)
	}
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}
}

func TestParseTSListKeepsOriginalOrderWithoutStart(t *testing.T) {
	segments, err := ParseTSList([]byte(`
https://example.com/2.ts?type=mpegts
https://example.com/1.ts?type=mpegts
`))
	if err != nil {
		t.Fatalf("ParseTSList returned error: %v", err)
	}
	if segments[0].URL != "https://example.com/2.ts?type=mpegts" || segments[1].URL != "https://example.com/1.ts?type=mpegts" {
		t.Fatalf("segments not in original order: %#v", segments)
	}
}

func TestParseTSListRejectsNonTSURL(t *testing.T) {
	_, err := ParseTSList([]byte("https://example.com/video.mp4\n"))
	if err == nil {
		t.Fatal("expected error for non-TS URL")
	}
}
