package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any env vars that might interfere
	os.Unsetenv("PORT")
	os.Unsetenv("ENV")
	os.Unsetenv("GCP_PROJECT_ID")
	os.Unsetenv("GCS_BUCKET_NAME")
	os.Unsetenv("GCS_SIGNED_URL_EXPIRY")
	os.Unsetenv("FFMPEG_PATH")
	os.Unsetenv("PROXY_RESOLUTION")
	os.Unsetenv("PROXY_BITRATE")

	cfg := Load()

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.Env != "development" {
		t.Errorf("Env = %q, want %q", cfg.Env, "development")
	}
	if cfg.GCPProjectID != "" {
		t.Errorf("GCPProjectID = %q, want empty", cfg.GCPProjectID)
	}
	if cfg.GCSBucketName != "cloudcut-media" {
		t.Errorf("GCSBucketName = %q, want %q", cfg.GCSBucketName, "cloudcut-media")
	}
	if cfg.GCSSignedURLExpiry != 3600 {
		t.Errorf("GCSSignedURLExpiry = %d, want 3600", cfg.GCSSignedURLExpiry)
	}
	if cfg.FFmpegPath != "ffmpeg" {
		t.Errorf("FFmpegPath = %q, want %q", cfg.FFmpegPath, "ffmpeg")
	}
	if cfg.ProxyResolution != 720 {
		t.Errorf("ProxyResolution = %d, want 720", cfg.ProxyResolution)
	}
	if cfg.ProxyBitrate != "1M" {
		t.Errorf("ProxyBitrate = %q, want %q", cfg.ProxyBitrate, "1M")
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("ENV", "production")
	t.Setenv("GCP_PROJECT_ID", "my-project")
	t.Setenv("GCS_BUCKET_NAME", "my-bucket")
	t.Setenv("GCS_SIGNED_URL_EXPIRY", "7200")
	t.Setenv("FFMPEG_PATH", "/usr/local/bin/ffmpeg")
	t.Setenv("PROXY_RESOLUTION", "480")
	t.Setenv("PROXY_BITRATE", "500k")

	cfg := Load()

	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.Env != "production" {
		t.Errorf("Env = %q, want %q", cfg.Env, "production")
	}
	if cfg.GCPProjectID != "my-project" {
		t.Errorf("GCPProjectID = %q, want %q", cfg.GCPProjectID, "my-project")
	}
	if cfg.GCSBucketName != "my-bucket" {
		t.Errorf("GCSBucketName = %q, want %q", cfg.GCSBucketName, "my-bucket")
	}
	if cfg.GCSSignedURLExpiry != 7200 {
		t.Errorf("GCSSignedURLExpiry = %d, want 7200", cfg.GCSSignedURLExpiry)
	}
	if cfg.FFmpegPath != "/usr/local/bin/ffmpeg" {
		t.Errorf("FFmpegPath = %q, want %q", cfg.FFmpegPath, "/usr/local/bin/ffmpeg")
	}
	if cfg.ProxyResolution != 480 {
		t.Errorf("ProxyResolution = %d, want 480", cfg.ProxyResolution)
	}
	if cfg.ProxyBitrate != "500k" {
		t.Errorf("ProxyBitrate = %q, want %q", cfg.ProxyBitrate, "500k")
	}
}

func TestLoad_InvalidIntFallsBackToDefault(t *testing.T) {
	t.Setenv("PORT", "not-a-number")
	t.Setenv("PROXY_RESOLUTION", "abc")

	cfg := Load()

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080 (fallback)", cfg.Port)
	}
	if cfg.ProxyResolution != 720 {
		t.Errorf("ProxyResolution = %d, want 720 (fallback)", cfg.ProxyResolution)
	}
}
