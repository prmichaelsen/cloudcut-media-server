package retry

import (
	"context"
	"fmt"
	"time"
)

// Config configures retry behavior.
type Config struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

// DefaultConfig returns default retry configuration.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		InitialWait: 500 * time.Millisecond,
		MaxWait:     5 * time.Second,
		Multiplier:  2.0,
	}
}

// Do executes fn with exponential backoff retry on transient failures.
// isRetryable determines if the error should trigger a retry.
func Do(ctx context.Context, cfg Config, fn func() error, isRetryable func(error) bool) error {
	var lastErr error
	wait := cfg.InitialWait

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// Check if error is retryable
		if !isRetryable(lastErr) {
			return lastErr
		}

		// Don't wait after last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Check context before sleeping
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}

		// Exponential backoff
		wait = time.Duration(float64(wait) * cfg.Multiplier)
		if wait > cfg.MaxWait {
			wait = cfg.MaxWait
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// IsGCSTransient checks if an error is a transient GCS error (5xx, timeout).
func IsGCSTransient(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Check for common transient error patterns
	return containsAny(errStr, []string{
		"500",
		"502",
		"503",
		"504",
		"timeout",
		"deadline exceeded",
		"connection reset",
		"temporary failure",
	})
}

// IsFFmpegTransient checks if an FFmpeg error is transient.
func IsFFmpegTransient(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Temporary file system issues are retryable
	return containsAny(errStr, []string{
		"no space left",
		"disk full",
		"temporary",
	})
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
