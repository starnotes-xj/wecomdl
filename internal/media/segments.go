package media

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

type Segment struct {
	URL   string
	Start int64
	End   int64
	Index int
}

func ParseTSList(data []byte) ([]Segment, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	seen := map[string]bool{}
	var segments []Segment
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		if seen[raw] {
			continue
		}
		segment, err := parseSegment(raw, len(segments))
		if err != nil {
			return nil, fmt.Errorf("第 %d 行 TS URL 无效: %w", lineNumber, err)
		}
		seen[raw] = true
		segments = append(segments, segment)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(segments) == 0 {
		return nil, fmt.Errorf("TS 列表为空")
	}
	if allSegmentsHaveStart(segments) {
		slices.SortStableFunc(segments, func(a, b Segment) int {
			if a.Start != b.Start {
				return compareInt64(a.Start, b.Start)
			}
			if a.End != b.End {
				return compareInt64(a.End, b.End)
			}
			return a.Index - b.Index
		})
	}
	return segments, nil
}

func parseSegment(raw string, index int) (Segment, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return Segment{}, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Segment{}, fmt.Errorf("只支持 http/https URL")
	}
	query := parsed.Query()
	if !strings.Contains(strings.ToLower(parsed.Path), ".ts") && !strings.EqualFold(query.Get("type"), "mpegts") {
		return Segment{}, fmt.Errorf("看起来不是 TS 分片")
	}
	segment := Segment{URL: raw, Start: -1, End: -1, Index: index}
	if value := query.Get("start"); value != "" {
		start, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return Segment{}, fmt.Errorf("start 参数无效")
		}
		segment.Start = start
	}
	if value := query.Get("end"); value != "" {
		end, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return Segment{}, fmt.Errorf("end 参数无效")
		}
		segment.End = end
	}
	return segment, nil
}

func allSegmentsHaveStart(segments []Segment) bool {
	for _, segment := range segments {
		if segment.Start < 0 {
			return false
		}
	}
	return true
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
