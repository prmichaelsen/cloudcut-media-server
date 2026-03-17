package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port              int
	Env               string
	GCPProjectID      string
	GCSBucketName     string
	GCSSignedURLExpiry int
	FFmpegPath        string
	ProxyResolution   int
	ProxyBitrate      string
}

func Load() *Config {
	return &Config{
		Port:              getEnvInt("PORT", 8080),
		Env:               getEnv("ENV", "development"),
		GCPProjectID:      getEnv("GCP_PROJECT_ID", ""),
		GCSBucketName:     getEnv("GCS_BUCKET_NAME", "cloudcut-media"),
		GCSSignedURLExpiry: getEnvInt("GCS_SIGNED_URL_EXPIRY", 3600),
		FFmpegPath:        getEnv("FFMPEG_PATH", "ffmpeg"),
		ProxyResolution:   getEnvInt("PROXY_RESOLUTION", 720),
		ProxyBitrate:      getEnv("PROXY_BITRATE", "1M"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}
