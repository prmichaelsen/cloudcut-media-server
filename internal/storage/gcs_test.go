package storage

import (
	"testing"
)

func TestSourcePath(t *testing.T) {
	tests := []struct {
		mediaID string
		ext     string
		want    string
	}{
		{"abc-123", "mp4", "sources/abc-123/original.mp4"},
		{"def-456", "mov", "sources/def-456/original.mov"},
		{"ghi-789", "webm", "sources/ghi-789/original.webm"},
	}

	for _, tt := range tests {
		got := SourcePath(tt.mediaID, tt.ext)
		if got != tt.want {
			t.Errorf("SourcePath(%q, %q) = %q, want %q", tt.mediaID, tt.ext, got, tt.want)
		}
	}
}

func TestProxyPath(t *testing.T) {
	tests := []struct {
		mediaID string
		want    string
	}{
		{"abc-123", "proxies/abc-123/proxy.mp4"},
		{"def-456", "proxies/def-456/proxy.mp4"},
	}

	for _, tt := range tests {
		got := ProxyPath(tt.mediaID)
		if got != tt.want {
			t.Errorf("ProxyPath(%q) = %q, want %q", tt.mediaID, got, tt.want)
		}
	}
}

func TestExportPath(t *testing.T) {
	path := ExportPath("session-1")

	// Should match pattern: exports/session-1/{timestamp}.mp4
	if len(path) < len("exports/session-1/") {
		t.Fatalf("ExportPath too short: %q", path)
	}

	prefix := "exports/session-1/"
	if path[:len(prefix)] != prefix {
		t.Errorf("ExportPath prefix = %q, want %q", path[:len(prefix)], prefix)
	}

	if path[len(path)-4:] != ".mp4" {
		t.Errorf("ExportPath suffix = %q, want .mp4", path[len(path)-4:])
	}
}
