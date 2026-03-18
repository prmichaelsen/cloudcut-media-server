package validation

import (
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/prmichaelsen/cloudcut-media-server/internal/errors"
)

const (
	MaxUploadSize   = 5 << 30 // 5GB
	MaxEDLSize      = 10 << 20 // 10MB
)

var AllowedContentTypes = map[string]bool{
	"video/mp4":        true,
	"video/quicktime":  true,
	"video/webm":       true,
	"video/x-matroska": true,
	"video/x-msvideo":  true,
}

// ValidateUpload validates an uploaded file.
func ValidateUpload(header *multipart.FileHeader) *errors.AppError {
	// Check size
	if header.Size > MaxUploadSize {
		return errors.NewWithDetails(
			errors.ErrUploadTooLarge,
			"file exceeds maximum upload size of 5GB",
			false,
			map[string]interface{}{
				"size":     header.Size,
				"maxSize": MaxUploadSize,
			},
		)
	}

	// Check content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = InferContentType(header.Filename)
	}

	if !AllowedContentTypes[contentType] {
		return errors.NewWithDetails(
			errors.ErrInvalidContentType,
			"unsupported video format",
			false,
			map[string]interface{}{
				"contentType": contentType,
				"allowed":     []string{"mp4", "mov", "webm", "mkv", "avi"},
			},
		)
	}

	return nil
}

// InferContentType infers content type from filename extension.
func InferContentType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".avi":
		return "video/x-msvideo"
	default:
		return "application/octet-stream"
	}
}
