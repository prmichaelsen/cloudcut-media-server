package api

import (
	"testing"
)

func TestInferContentType(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"video.mp4", "video/mp4"},
		{"video.MP4", "video/mp4"},
		{"clip.mov", "video/quicktime"},
		{"clip.MOV", "video/quicktime"},
		{"stream.webm", "video/webm"},
		{"movie.mkv", "video/x-matroska"},
		{"old.avi", "video/x-msvideo"},
		{"unknown.xyz", "application/octet-stream"},
		{"noext", "application/octet-stream"},
	}

	for _, tt := range tests {
		got := inferContentType(tt.filename)
		if got != tt.want {
			t.Errorf("inferContentType(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}
