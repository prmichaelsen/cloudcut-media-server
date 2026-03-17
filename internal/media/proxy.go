package media

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/prmichaelsen/cloudcut-media-server/internal/config"
	"github.com/prmichaelsen/cloudcut-media-server/internal/storage"
	"github.com/prmichaelsen/cloudcut-media-server/pkg/models"
)

type ProxyGenerator struct {
	gcs    *storage.GCSClient
	ffmpeg FFmpegConfig
}

func NewProxyGenerator(gcs *storage.GCSClient, cfg *config.Config) *ProxyGenerator {
	return &ProxyGenerator{
		gcs: gcs,
		ffmpeg: FFmpegConfig{
			Path:       cfg.FFmpegPath,
			Resolution: cfg.ProxyResolution,
			Bitrate:    cfg.ProxyBitrate,
		},
	}
}

// GenerateProxy downloads the source from GCS, runs FFmpeg to create a proxy,
// uploads the proxy back to GCS, and updates the media record.
func (p *ProxyGenerator) GenerateProxy(ctx context.Context, media *models.Media) error {
	tmpDir, err := os.MkdirTemp("", "cloudcut-proxy-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download source from GCS
	inputPath := filepath.Join(tmpDir, "source"+filepath.Ext(media.OriginalFilename))
	if err := p.downloadToFile(ctx, media.GCSSourcePath, inputPath); err != nil {
		return fmt.Errorf("download source: %w", err)
	}

	// Generate proxy via FFmpeg
	outputPath := filepath.Join(tmpDir, "proxy.mp4")
	args := BuildProxyArgs(inputPath, outputPath, p.ffmpeg)

	result, err := RunFFmpeg(ctx, p.ffmpeg.Path, args, 0)
	if err != nil {
		return fmt.Errorf("ffmpeg proxy generation: %w", err)
	}

	log.Printf("proxy generated for %s in %v", media.ID, result.Duration)

	// Upload proxy to GCS
	proxyPath := storage.ProxyPath(media.ID)
	proxyFile, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("open proxy file: %w", err)
	}
	defer proxyFile.Close()

	if err := p.gcs.Upload(ctx, proxyPath, proxyFile); err != nil {
		return fmt.Errorf("upload proxy: %w", err)
	}

	// Update media record
	media.GCSProxyPath = proxyPath
	media.Status = models.MediaStatusReady

	log.Printf("proxy uploaded for %s at %s", media.ID, proxyPath)

	return nil
}

func (p *ProxyGenerator) downloadToFile(ctx context.Context, gcsPath, localPath string) error {
	rc, err := p.gcs.Download(ctx, gcsPath)
	if err != nil {
		return err
	}
	defer rc.Close()

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", localPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, rc); err != nil {
		return fmt.Errorf("write file %s: %w", localPath, err)
	}

	return nil
}
