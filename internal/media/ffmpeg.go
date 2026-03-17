package media

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

type FFmpegConfig struct {
	Path       string
	Resolution int
	Bitrate    string
}

type FFmpegResult struct {
	Duration time.Duration
	Stderr   string
}

// BuildProxyArgs returns FFmpeg arguments for proxy generation.
func BuildProxyArgs(input, output string, cfg FFmpegConfig) []string {
	scaleFilter := fmt.Sprintf("scale=-2:%d", cfg.Resolution)

	return []string{
		"-i", input,
		"-vf", scaleFilter,
		"-c:v", "libx264",
		"-preset", "fast",
		"-b:v", cfg.Bitrate,
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-y",
		output,
	}
}

// RunFFmpeg executes an FFmpeg command with the given arguments.
// It returns the result or an error if FFmpeg fails or times out.
func RunFFmpeg(ctx context.Context, ffmpegPath string, args []string, timeout time.Duration) (*FFmpegResult, error) {
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &FFmpegResult{
		Duration: duration,
		Stderr:   stderr.String(),
	}

	if ctx.Err() == context.DeadlineExceeded {
		return result, fmt.Errorf("ffmpeg timed out after %v", timeout)
	}

	if err != nil {
		return result, fmt.Errorf("ffmpeg failed (exit %v): %s", err, stderr.String())
	}

	return result, nil
}

// Probe returns the duration of a media file in seconds using ffprobe.
func Probe(ctx context.Context, ffmpegPath, inputPath string) (float64, error) {
	probePath := ffmpegPath
	// Try ffprobe alongside ffmpeg
	if probePath == "ffmpeg" {
		probePath = "ffprobe"
	} else {
		probePath = ffmpegPath[:len(ffmpegPath)-len("ffmpeg")] + "ffprobe"
	}

	cmd := exec.CommandContext(ctx, probePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputPath,
	)

	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	duration, err := strconv.ParseFloat(string(bytes.TrimSpace(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration: %w", err)
	}

	return duration, nil
}
