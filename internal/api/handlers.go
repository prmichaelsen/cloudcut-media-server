package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prmichaelsen/cloudcut-media-server/internal/storage"
	"github.com/prmichaelsen/cloudcut-media-server/pkg/models"
)

const maxUploadSize = 5 << 30 // 5GB

var allowedContentTypes = map[string]bool{
	"video/mp4":       true,
	"video/quicktime": true,
	"video/webm":      true,
	"video/x-matroska": true,
	"video/x-msvideo": true,
}

type Handlers struct {
	gcs   *storage.GCSClient
	media map[string]*models.Media // in-memory store for MVP
}

func NewHandlers(gcs *storage.GCSClient) *Handlers {
	return &Handlers{
		gcs:   gcs,
		media: make(map[string]*models.Media),
	}
}

func (h *Handlers) HandleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "UPLOAD_TOO_LARGE", "file exceeds maximum upload size")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_UPLOAD", "missing or invalid file field")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = inferContentType(header.Filename)
	}
	if !allowedContentTypes[contentType] {
		writeError(w, http.StatusBadRequest, "INVALID_CONTENT_TYPE",
			fmt.Sprintf("unsupported content type: %s", contentType))
		return
	}

	mediaID := uuid.New().String()
	ext := strings.TrimPrefix(filepath.Ext(header.Filename), ".")
	if ext == "" {
		ext = "mp4"
	}

	gcsPath := storage.SourcePath(mediaID, ext)

	media := &models.Media{
		ID:               mediaID,
		OriginalFilename: header.Filename,
		ContentType:      contentType,
		Size:             header.Size,
		GCSSourcePath:    gcsPath,
		Status:           models.MediaStatusUploading,
		CreatedAt:        time.Now(),
	}
	h.media[mediaID] = media

	if err := h.gcs.Upload(r.Context(), gcsPath, file); err != nil {
		media.Status = models.MediaStatusError
		media.Error = err.Error()
		log.Printf("upload failed for %s: %v", mediaID, err)
		writeError(w, http.StatusInternalServerError, "UPLOAD_FAILED", "failed to upload file to storage")
		return
	}

	media.Status = models.MediaStatusProcessing

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       media.ID,
		"filename": media.OriginalFilename,
		"size":     media.Size,
		"status":   media.Status,
		"gcsPath":  media.GCSSourcePath,
	})
}

func (h *Handlers) HandleGetMedia(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("id")
	media, ok := h.media[mediaID]
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}

func (h *Handlers) HandleGetSignedURL(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("id")
	media, ok := h.media[mediaID]
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media not found")
		return
	}

	url, err := h.gcs.SignedURL(media.GCSSourcePath, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SIGN_FAILED", "failed to generate signed URL")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url": url,
	})
}

func (h *Handlers) HandleGetProxyURL(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("id")
	media, ok := h.media[mediaID]
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media not found")
		return
	}

	if media.GCSProxyPath == "" {
		writeError(w, http.StatusNotFound, "PROXY_NOT_READY", "proxy not yet generated")
		return
	}

	url, err := h.gcs.SignedURL(media.GCSProxyPath, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SIGN_FAILED", "failed to generate signed URL")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url": url,
	})
}

// GetMedia returns the media entry for a given ID (used by other packages).
func (h *Handlers) GetMedia(mediaID string) (*models.Media, bool) {
	m, ok := h.media[mediaID]
	return m, ok
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

func inferContentType(filename string) string {
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
