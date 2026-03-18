package progress

import (
	"sync"
	"time"
)

// Reporter throttles and reports progress updates to avoid flooding consumers.
type Reporter struct {
	mu            sync.Mutex
	lastReport    time.Time
	minInterval   time.Duration
	reportFunc    ReportFunc
	currentStage  string
	lastProgress  float64
}

// ReportFunc is called with throttled progress updates.
// Implementations should send updates via WebSocket or other channels.
type ReportFunc func(percent float64, fps int, speed string, eta int, stage string)

// NewReporter creates a new progress reporter with throttling.
// minInterval is the minimum time between progress updates (e.g., 500ms for 2 updates/second).
func NewReporter(reportFunc ReportFunc, minInterval time.Duration) *Reporter {
	if minInterval == 0 {
		minInterval = 500 * time.Millisecond // Default: 2 updates per second
	}

	return &Reporter{
		reportFunc:  reportFunc,
		minInterval: minInterval,
		lastReport:  time.Now().Add(-minInterval), // Allow immediate first report
	}
}

// Report sends a progress update if enough time has elapsed since the last update.
// This implements throttling to avoid flooding the WebSocket.
func (r *Reporter) Report(prog Progress, stage string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastReport)

	// Check if we should throttle this update
	// Always send if stage changed or progress reached 100%
	shouldReport := elapsed >= r.minInterval ||
		stage != r.currentStage ||
		prog.Progress >= 100

	if !shouldReport {
		return
	}

	// Update state
	r.lastReport = now
	r.currentStage = stage
	r.lastProgress = prog.Progress

	// Calculate speed string
	speedStr := ""
	if prog.Speed > 0 {
		speedStr = formatSpeed(prog.Speed)
	}

	// Calculate ETA
	eta := 0
	if prog.Speed > 0 {
		// Estimate remaining time based on speed
		// This is a rough estimate; more accurate ETA would require total duration
		eta = int(float64(100-prog.Progress) / prog.Speed)
	}

	// Send update
	r.reportFunc(
		prog.Progress,
		int(prog.FPS),
		speedStr,
		eta,
		stage,
	)
}

// ReportStage sends a stage transition update immediately (not throttled).
func (r *Reporter) ReportStage(stage string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if stage == r.currentStage {
		return
	}

	r.currentStage = stage
	r.lastReport = time.Now()

	// Report stage with last known progress
	r.reportFunc(r.lastProgress, 0, "", 0, stage)
}

// formatSpeed converts speed multiplier to human-readable string (e.g., "2.1x").
func formatSpeed(speed float64) string {
	if speed < 0.01 {
		return "0.0x"
	}
	if speed >= 100 {
		return "99.9x"
	}
	return formatFloat(speed, 1) + "x"
}

// formatFloat formats a float to n decimal places.
func formatFloat(f float64, decimals int) string {
	result := ""
	switch decimals {
	case 0:
		result = formatInt(int(f))
	case 1:
		result = formatFloat1(f)
	default:
		// Fallback for other decimal counts
		result = formatFloat1(f)
	}
	return result
}

func formatInt(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

func formatFloat1(f float64) string {
	integer := int(f)
	decimal := int((f - float64(integer)) * 10)
	if decimal < 0 {
		decimal = -decimal
	}

	result := ""
	if integer >= 10 {
		result += string(rune('0' + integer/10))
	}
	result += string(rune('0' + integer%10))
	result += "."
	result += string(rune('0' + decimal))

	return result
}
