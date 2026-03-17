package render

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/prmichaelsen/cloudcut-media-server/internal/edl"
)

// JobStatus represents the state of a render job.
type JobStatus string

const (
	JobStatusQueued     JobStatus = "queued"
	JobStatusProcessing JobStatus = "processing"
	JobStatusComplete   JobStatus = "complete"
	JobStatusError      JobStatus = "error"
)

// Job represents a single render job.
type Job struct {
	ID         string
	EDL        *edl.EDL
	Status     JobStatus
	Progress   float64 // 0.0 to 100.0
	OutputPath string  // GCS path to rendered output
	Error      string
	CreatedAt  time.Time
	StartedAt  time.Time
	CompletedAt time.Time
	SessionID  string
}

// NewJob creates a new render job from a validated EDL.
func NewJob(sessionID string, parsedEDL *edl.EDL) *Job {
	return &Job{
		ID:        uuid.New().String(),
		EDL:       parsedEDL,
		Status:    JobStatusQueued,
		Progress:  0,
		CreatedAt: time.Now(),
		SessionID: sessionID,
	}
}

// ProgressCallback is called periodically during rendering to report progress.
type ProgressCallback func(progress float64, fps int, speed string, eta int, stage string)

// Renderer orchestrates FFmpeg rendering from an EDL.
type Renderer struct {
	gcs     GCSClient
	ffmpeg  FFmpegClient
	storage JobStorage
}

// GCSClient interface for storage operations (simplified for MVP).
type GCSClient interface {
	SignedURL(path string, expiry time.Duration) (string, error)
}

// FFmpegClient interface for FFmpeg operations.
type FFmpegClient interface {
	BuildRenderCommand(edl *edl.EDL, inputPaths []string, outputPath string) ([]string, error)
	Run(ctx context.Context, args []string, progressCb ProgressCallback) error
}

// JobStorage interface for job persistence.
type JobStorage interface {
	Save(job *Job) error
	Get(jobID string) (*Job, error)
	List() ([]*Job, error)
}

// NewRenderer creates a new Renderer.
func NewRenderer(gcs GCSClient, ffmpeg FFmpegClient, storage JobStorage) *Renderer {
	return &Renderer{
		gcs:     gcs,
		ffmpeg:  ffmpeg,
		storage: storage,
	}
}

// Submit creates and queues a new render job.
func (r *Renderer) Submit(sessionID string, parsedEDL *edl.EDL) (*Job, error) {
	job := NewJob(sessionID, parsedEDL)
	if err := r.storage.Save(job); err != nil {
		return nil, fmt.Errorf("save job: %w", err)
	}
	return job, nil
}

// Render executes a render job.
func (r *Renderer) Render(ctx context.Context, job *Job, progressCb ProgressCallback) error {
	job.Status = JobStatusProcessing
	job.StartedAt = time.Now()
	r.storage.Save(job)

	// Collect input file paths for all media referenced in EDL
	inputPaths, err := r.resolveMediaPaths(ctx, job.EDL)
	if err != nil {
		job.Status = JobStatusError
		job.Error = fmt.Sprintf("resolve media: %v", err)
		r.storage.Save(job)
		return fmt.Errorf("resolve media paths: %w", err)
	}

	// Generate output path
	ext := "mp4"
	if job.EDL.Output.Format == "webm" {
		ext = "webm"
	}
	outputPath := fmt.Sprintf("exports/%s/output.%s", job.ID, ext)
	job.OutputPath = outputPath

	// Build FFmpeg command
	args, err := r.ffmpeg.BuildRenderCommand(job.EDL, inputPaths, outputPath)
	if err != nil {
		job.Status = JobStatusError
		job.Error = fmt.Sprintf("build command: %v", err)
		r.storage.Save(job)
		return fmt.Errorf("build ffmpeg command: %w", err)
	}

	// Execute FFmpeg
	if err := r.ffmpeg.Run(ctx, args, progressCb); err != nil {
		job.Status = JobStatusError
		job.Error = fmt.Sprintf("ffmpeg failed: %v", err)
		r.storage.Save(job)
		return fmt.Errorf("ffmpeg execution: %w", err)
	}

	job.Status = JobStatusComplete
	job.Progress = 100
	job.CompletedAt = time.Now()
	r.storage.Save(job)

	return nil
}

// GetJob retrieves a job by ID.
func (r *Renderer) GetJob(jobID string) (*Job, error) {
	return r.storage.Get(jobID)
}

// resolveMediaPaths collects all media file paths referenced in the EDL.
func (r *Renderer) resolveMediaPaths(ctx context.Context, edl *edl.EDL) ([]string, error) {
	mediaIDs := make(map[string]bool)
	for _, track := range edl.Timeline.Tracks {
		for _, clip := range track.Clips {
			mediaIDs[clip.MediaID] = true
		}
	}

	var paths []string
	for mediaID := range mediaIDs {
		// TODO: lookup actual media path from media service
		// For now, assume source path format
		path := fmt.Sprintf("sources/%s/original.mp4", mediaID)
		paths = append(paths, path)
	}

	return paths, nil
}
