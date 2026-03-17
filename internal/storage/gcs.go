package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"github.com/prmichaelsen/cloudcut-media-server/internal/config"
)

type GCSClient struct {
	client     *storage.Client
	bucket     string
	urlExpiry  time.Duration
}

func NewGCSClient(ctx context.Context, cfg *config.Config) (*GCSClient, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create gcs client: %w", err)
	}

	return &GCSClient{
		client:    client,
		bucket:    cfg.GCSBucketName,
		urlExpiry: time.Duration(cfg.GCSSignedURLExpiry) * time.Second,
	}, nil
}

func (g *GCSClient) Upload(ctx context.Context, objectPath string, reader io.Reader) error {
	wc := g.client.Bucket(g.bucket).Object(objectPath).NewWriter(ctx)
	if _, err := io.Copy(wc, reader); err != nil {
		wc.Close()
		return fmt.Errorf("upload to %s: %w", objectPath, err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("close writer for %s: %w", objectPath, err)
	}
	return nil
}

func (g *GCSClient) Download(ctx context.Context, objectPath string) (io.ReadCloser, error) {
	rc, err := g.client.Bucket(g.bucket).Object(objectPath).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", objectPath, err)
	}
	return rc, nil
}

func (g *GCSClient) SignedURL(objectPath string, expiry time.Duration) (string, error) {
	if expiry == 0 {
		expiry = g.urlExpiry
	}

	url, err := g.client.Bucket(g.bucket).SignedURL(objectPath, &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(expiry),
	})
	if err != nil {
		return "", fmt.Errorf("sign url for %s: %w", objectPath, err)
	}
	return url, nil
}

func (g *GCSClient) Delete(ctx context.Context, objectPath string) error {
	if err := g.client.Bucket(g.bucket).Object(objectPath).Delete(ctx); err != nil {
		return fmt.Errorf("delete %s: %w", objectPath, err)
	}
	return nil
}

func (g *GCSClient) Close() error {
	return g.client.Close()
}

// SourcePath returns the GCS path for a source media file.
func SourcePath(mediaID, ext string) string {
	return fmt.Sprintf("sources/%s/original.%s", mediaID, ext)
}

// ProxyPath returns the GCS path for a proxy media file.
func ProxyPath(mediaID string) string {
	return fmt.Sprintf("proxies/%s/proxy.mp4", mediaID)
}

// ExportPath returns the GCS path for a rendered export.
func ExportPath(sessionID string) string {
	return fmt.Sprintf("exports/%s/%d.mp4", sessionID, time.Now().Unix())
}
