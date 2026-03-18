package progress

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"time"
)

// Progress represents FFmpeg progress state parsed from stderr.
type Progress struct {
	Frame    int           // Current frame number
	FPS      float64       // Current encoding FPS
	Bitrate  string        // Current bitrate (e.g., "2000kbits/s")
	Time     time.Duration // Current output time
	Speed    float64       // Encoding speed multiplier (e.g., 2.1x)
	Progress float64       // Percentage complete (0-100)
}

// Parser parses FFmpeg progress output from stderr.
type Parser struct {
	totalDuration time.Duration
	progressRegex *regexp.Regexp
}

// NewParser creates a new FFmpeg progress parser.
// totalDuration is the expected output duration for calculating percentage.
func NewParser(totalDuration time.Duration) *Parser {
	// FFmpeg progress output format:
	// frame=  123 fps= 45 q=28.0 size=    1024kB time=00:00:05.12 bitrate=1638.4kbits/s speed=1.23x
	return &Parser{
		totalDuration: totalDuration,
		progressRegex: regexp.MustCompile(`frame=\s*(\d+)\s+fps=\s*([\d.]+).*time=(\d+):(\d+):([\d.]+).*bitrate=\s*([\d.]+\w+/s).*speed=\s*([\d.]+)x`),
	}
}

// Parse reads FFmpeg stderr line by line and calls the callback with parsed progress.
// Returns when the reader closes or context is cancelled.
func (p *Parser) Parse(r io.Reader, callback func(Progress)) error {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()

		// Parse progress line
		if prog := p.parseLine(line); prog != nil {
			callback(*prog)
		}
	}

	return scanner.Err()
}

// parseLine extracts progress data from a single FFmpeg output line.
func (p *Parser) parseLine(line string) *Progress {
	matches := p.progressRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil
	}

	// Extract fields
	frame, _ := strconv.Atoi(matches[1])
	fps, _ := strconv.ParseFloat(matches[2], 64)
	hours, _ := strconv.Atoi(matches[3])
	minutes, _ := strconv.Atoi(matches[4])
	seconds, _ := strconv.ParseFloat(matches[5], 64)
	bitrate := matches[6]
	speed, _ := strconv.ParseFloat(matches[7], 64)

	// Calculate current time
	currentTime := time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds*float64(time.Second))

	// Calculate percentage complete
	var percent float64
	if p.totalDuration > 0 {
		percent = (float64(currentTime) / float64(p.totalDuration)) * 100
		if percent > 100 {
			percent = 100
		}
	}

	return &Progress{
		Frame:    frame,
		FPS:      fps,
		Bitrate:  bitrate,
		Time:     currentTime,
		Speed:    speed,
		Progress: percent,
	}
}

// EstimateETA calculates estimated time remaining based on speed multiplier.
func (p *Parser) EstimateETA(currentTime time.Duration, speed float64) int {
	if speed <= 0 || p.totalDuration <= 0 {
		return 0
	}

	remaining := p.totalDuration - currentTime
	if remaining <= 0 {
		return 0
	}

	// ETA in seconds = remaining time / speed
	eta := float64(remaining) / (speed * float64(time.Second))
	return int(eta)
}

// ParseSimple is a convenience function for parsing a single line.
// Returns nil if the line doesn't contain progress data.
func ParseSimple(line string, totalDuration time.Duration) *Progress {
	parser := NewParser(totalDuration)
	return parser.parseLine(line)
}
