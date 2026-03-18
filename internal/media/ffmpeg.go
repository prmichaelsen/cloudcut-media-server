package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"time"

	"github.com/prmichaelsen/cloudcut-media-server/internal/progress"
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

// ProgressCallback is called periodically during FFmpeg execution with progress updates.
type ProgressCallback func(percent float64, fps int, speed string, eta int, stage string)

// RunFFmpeg executes an FFmpeg command with the given arguments.
// It returns the result or an error if FFmpeg fails or times out.
func RunFFmpeg(ctx context.Context, ffmpegPath string, args []string, timeout time.Duration) (*FFmpegResult, error) {
	return RunFFmpegWithProgress(ctx, ffmpegPath, args, timeout, nil)
}

// RunFFmpegWithProgress executes FFmpeg with progress tracking.
// progressCb is called periodically with progress updates (can be nil).
func RunFFmpegWithProgress(ctx context.Context, ffmpegPath string, args []string, timeout time.Duration, progressCb ProgressCallback) (*FFmpegResult, error) {
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)

	// Capture stderr for both progress parsing and error reporting
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	start := time.Now()

	// Start FFmpeg
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	// Collect stderr and parse progress
	var stderrBuf bytes.Buffer
	done := make(chan error, 1)

	go func() {
		if progressCb != nil {
			// Parse progress and collect stderr
			parser := progress.NewParser(0) // Duration unknown for proxy generation
			buf := bytes.NewBuffer(nil)
			teeReader := io.TeeReader(stderrPipe, buf)

			parser.Parse(teeReader, func(prog progress.Progress) {
				speedStr := ""
				if prog.Speed > 0 {
					speedStr = fmt.Sprintf("%.1fx", prog.Speed)
				}
				progressCb(prog.Progress, int(prog.FPS), speedStr, 0, "rendering")
			})

			stderrBuf.Write(buf.Bytes())
			done <- nil
		} else {
			// Just collect stderr
			io.Copy(&stderrBuf, stderrPipe)
			done <- nil
		}
	}()

	// Wait for FFmpeg to complete
	cmdErr := cmd.Wait()
	duration := time.Since(start)

	// Wait for stderr collection to finish
	<-done

	result := &FFmpegResult{
		Duration: duration,
		Stderr:   stderrBuf.String(),
	}

	if ctx.Err() == context.DeadlineExceeded {
		return result, fmt.Errorf("ffmpeg timed out after %v", timeout)
	}

	if cmdErr != nil {
		return result, fmt.Errorf("ffmpeg failed (exit %v): %s", cmdErr, stderrBuf.String())
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
