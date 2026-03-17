package media

import (
	"testing"
)

func TestBuildProxyArgs(t *testing.T) {
	cfg := FFmpegConfig{
		Path:       "ffmpeg",
		Resolution: 720,
		Bitrate:    "1M",
	}

	args := BuildProxyArgs("/tmp/input.mp4", "/tmp/output.mp4", cfg)

	expected := []string{
		"-i", "/tmp/input.mp4",
		"-vf", "scale=-2:720",
		"-c:v", "libx264",
		"-preset", "fast",
		"-b:v", "1M",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-y",
		"/tmp/output.mp4",
	}

	if len(args) != len(expected) {
		t.Fatalf("got %d args, want %d", len(args), len(expected))
	}

	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("arg[%d] = %q, want %q", i, arg, expected[i])
		}
	}
}

func TestBuildProxyArgs_DifferentResolution(t *testing.T) {
	cfg := FFmpegConfig{
		Path:       "ffmpeg",
		Resolution: 480,
		Bitrate:    "500k",
	}

	args := BuildProxyArgs("in.mov", "out.mp4", cfg)

	// Check scale filter has correct resolution
	found := false
	for i, arg := range args {
		if arg == "-vf" && i+1 < len(args) {
			if args[i+1] != "scale=-2:480" {
				t.Errorf("scale filter = %q, want %q", args[i+1], "scale=-2:480")
			}
			found = true
		}
		if arg == "-b:v" && i+1 < len(args) {
			if args[i+1] != "500k" {
				t.Errorf("bitrate = %q, want %q", args[i+1], "500k")
			}
		}
	}
	if !found {
		t.Error("scale filter not found in args")
	}
}
