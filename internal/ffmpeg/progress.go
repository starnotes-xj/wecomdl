package ffmpeg

import (
	"strconv"
	"strings"
	"time"
)

type Progress struct {
	Frame     int64
	FPS       float64
	Bitrate   string
	TotalSize int64
	OutTime   time.Duration
	Speed     string
	Done      bool
}

type ProgressParser struct {
	current Progress
}

func (p *ProgressParser) ParseLine(line string) (Progress, bool) {
	key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
	if !ok {
		return Progress{}, false
	}

	switch key {
	case "frame":
		p.current.Frame = parseInt64(value, p.current.Frame)
	case "fps":
		p.current.FPS = parseFloat64(value, p.current.FPS)
	case "bitrate":
		p.current.Bitrate = value
	case "total_size":
		p.current.TotalSize = parseInt64(value, p.current.TotalSize)
	case "out_time_ms", "out_time_us":
		microseconds := parseInt64(value, 0)
		if microseconds > 0 {
			p.current.OutTime = time.Duration(microseconds) * time.Microsecond
		}
	case "out_time":
		if d, err := parseOutTime(value); err == nil {
			p.current.OutTime = d
		}
	case "speed":
		p.current.Speed = value
	case "progress":
		p.current.Done = value == "end"
		return p.current, true
	}
	return Progress{}, false
}

func parseInt64(raw string, fallback int64) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func parseFloat64(raw string, fallback float64) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return fallback
	}
	return value
}

func parseOutTime(raw string) (time.Duration, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 3 {
		return 0, strconv.ErrSyntax
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	seconds, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds*float64(time.Second)), nil
}
