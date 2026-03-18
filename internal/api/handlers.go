package api

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prmichaelsen/cloudcut-media-server/internal/errors"
	"github.com/prmichaelsen/cloudcut-media-server/internal/jobs"
	"github.com/prmichaelsen/cloudcut-media-server/internal/media"
	"github.com/prmichaelsen/cloudcut-media-server/internal/storage"
	"github.com/prmichaelsen/cloudcut-media-server/internal/validation"
	"github.com/prmichaelsen/cloudcut-media-server/pkg/models"
)


type Handlers struct {
	gcs        *storage.GCSClient
	proxy      *media.ProxyGenerator
	media      map[string]*models.Media // in-memory store for MVP
	jobManager *jobs.JobManager
	worker     *jobs.Worker
}

func NewHandlers(gcs *storage.GCSClient, proxy *media.ProxyGenerator, jobManager *jobs.JobManager, worker *jobs.Worker) *Handlers {
	return &Handlers{
		gcs:        gcs,
		proxy:      proxy,
		media:      make(map[string]*models.Media),
		jobManager: jobManager,
		worker:     worker,
	}
}

func (h *Handlers) HandleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, validation.MaxUploadSize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, errors.ErrUploadTooLarge, "file exceeds maximum upload size")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.ErrInvalidUpload, "missing or invalid file field")
		return
	}
	defer file.Close()

	// Validate upload
	if valErr := validation.ValidateUpload(header); valErr != nil {
		writeAppError(w, http.StatusBadRequest, valErr)
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = validation.InferContentType(header.Filename)
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

	// Create proxy generation job
	job := h.jobManager.CreateJob(jobs.JobTypeProxyGeneration, "", mediaID, nil)
	h.worker.Submit(job)

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

// SetWorker sets the worker reference (called after initialization).
func (h *Handlers) SetWorker(worker *jobs.Worker) {
	h.worker = worker
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeAppError(w, status, errors.New(code, message, false))
}

func writeAppError(w http.ResponseWriter, status int, err *errors.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err,
	})
}

