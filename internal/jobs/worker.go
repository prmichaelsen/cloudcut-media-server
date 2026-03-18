package jobs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/prmichaelsen/cloudcut-media-server/internal/media"
	"github.com/prmichaelsen/cloudcut-media-server/internal/progress"
	"github.com/prmichaelsen/cloudcut-media-server/internal/render"
	"github.com/prmichaelsen/cloudcut-media-server/internal/storage"
	"github.com/prmichaelsen/cloudcut-media-server/pkg/models"
)

// ProgressReporter is called when a job makes progress.
type ProgressReporter func(jobID string, percent float64, fps int, speed string, eta int, stage string)

// Worker processes jobs from the queue.
type Worker struct {
	manager   *JobManager
	gcs       *storage.GCSClient
	proxy     *media.ProxyGenerator
	renderer  *render.Renderer
	mediaStore MediaStore
	reporter  ProgressReporter

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	concurrency int
	jobChan     chan *Job
}

// MediaStore provides access to media records.
type MediaStore interface {
	GetMedia(mediaID string) (*models.Media, bool)
}

// NewWorker creates a new job worker.
func NewWorker(
	manager *JobManager,
	gcs *storage.GCSClient,
	proxy *media.ProxyGenerator,
	renderer *render.Renderer,
	mediaStore MediaStore,
	reporter ProgressReporter,
	concurrency int,
) *Worker {
	if concurrency <= 0 {
		concurrency = 2
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Worker{
		manager:     manager,
		gcs:         gcs,
		proxy:       proxy,
		renderer:    renderer,
		mediaStore:  mediaStore,
		reporter:    reporter,
		ctx:         ctx,
		cancel:      cancel,
		concurrency: concurrency,
		jobChan:     make(chan *Job, 100),
	}
}

// Start starts the worker pool.
func (w *Worker) Start() {
	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.workerLoop()
	}
	log.Printf("job worker started with %d workers", w.concurrency)
}

// Stop gracefully stops the worker pool.
func (w *Worker) Stop() {
	w.cancel()
	close(w.jobChan)
	w.wg.Wait()
	log.Printf("job worker stopped")
}

// Submit enqueues a job for processing.
func (w *Worker) Submit(job *Job) {
	select {
	case w.jobChan <- job:
		log.Printf("job %s queued (type=%s)", job.ID, job.Type)
	default:
		log.Printf("job queue full, rejecting job %s", job.ID)
		w.manager.UpdateJob(job.ID, func(j *Job) {
			j.Status = JobStatusFailed
			j.Error = "job queue full"
		})
	}
}

func (w *Worker) workerLoop() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		case job, ok := <-w.jobChan:
			if !ok {
				return
			}
			w.processJob(job)
		}
	}
}

func (w *Worker) processJob(job *Job) {
	log.Printf("processing job %s (type=%s)", job.ID, job.Type)

	// Update job status
	w.manager.UpdateJob(job.ID, func(j *Job) {
		j.Status = JobStatusDownloading
		j.StartedAt = time.Now()
	})

	var err error
	switch job.Type {
	case JobTypeProxyGeneration:
		err = w.processProxyJob(job)
	case JobTypeExportRender:
		err = w.processRenderJob(job)
	default:
		err = fmt.Errorf("unknown job type: %s", job.Type)
	}

	if err != nil {
		log.Printf("job %s failed: %v", job.ID, err)
		w.manager.UpdateJob(job.ID, func(j *Job) {
			j.Status = JobStatusFailed
			j.Error = err.Error()
			j.CompletedAt = time.Now()
		})

		// Report error to client
		if w.reporter != nil {
			w.reporter(job.ID, 0, 0, "", 0, "failed")
		}
	} else {
		log.Printf("job %s completed", job.ID)
	}
}

func (w *Worker) processProxyJob(job *Job) error {
	// Get media record
	media, ok := w.mediaStore.GetMedia(job.MediaID)
	if !ok {
		return fmt.Errorf("media not found: %s", job.MediaID)
	}

	// Create progress callback
	progressCb := w.makeProgressCallback(job)

	// Generate proxy
	if err := w.proxy.GenerateProxyWithProgress(w.ctx, media, progressCb); err != nil {
		return fmt.Errorf("proxy generation: %w", err)
	}

	// Update job status
	w.manager.UpdateJob(job.ID, func(j *Job) {
		j.Status = JobStatusComplete
		j.Progress = 100
		j.CompletedAt = time.Now()
		j.ResultURL = media.GCSProxyPath
	})

	// Report completion
	if w.reporter != nil {
		w.reporter(job.ID, 100, 0, "", 0, "complete")
	}

	return nil
}

func (w *Worker) processRenderJob(job *Job) error {
	// Create progress callback
	progressCb := func(progress float64, fps int, speed string, eta int, stage string) {
		// Update job state
		w.manager.UpdateJob(job.ID, func(j *Job) {
			j.Progress = progress
			j.FPS = fps
			j.Speed = speed
			j.ETA = eta
			j.Stage = stage

			// Map stage to status
			switch stage {
			case "downloading":
				j.Status = JobStatusDownloading
			case "rendering":
				j.Status = JobStatusRendering
			case "uploading":
				j.Status = JobStatusUploading
			case "complete":
				j.Status = JobStatusComplete
			}
		})

		// Report to client
		if w.reporter != nil {
			w.reporter(job.ID, progress, fps, speed, eta, stage)
		}
	}

	// Submit render job
	renderJob, err := w.renderer.Submit(job.SessionID, job.EDL)
	if err != nil {
		return fmt.Errorf("submit render: %w", err)
	}

	// Execute render
	if err := w.renderer.Render(w.ctx, renderJob, progressCb); err != nil {
		return fmt.Errorf("render: %w", err)
	}

	// Generate signed URL for download
	signedURL, err := w.gcs.SignedURL(renderJob.OutputPath, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("generate signed URL: %w", err)
	}

	// Update job with result
	w.manager.UpdateJob(job.ID, func(j *Job) {
		j.Status = JobStatusComplete
		j.Progress = 100
		j.ResultURL = signedURL
		j.CompletedAt = time.Now()
	})

	// Report completion
	if w.reporter != nil {
		w.reporter(job.ID, 100, 0, "", 0, "complete")
	}

	return nil
}

func (w *Worker) makeProgressCallback(job *Job) media.ProgressCallback {
	// Create throttled reporter
	reporter := progress.NewReporter(func(percent float64, fps int, speed string, eta int, stage string) {
		// Update job state
		w.manager.UpdateJob(job.ID, func(j *Job) {
			j.Progress = percent
			j.FPS = fps
			j.Speed = speed
			j.ETA = eta
			j.Stage = stage

			// Map stage to status
			switch stage {
			case "downloading":
				j.Status = JobStatusDownloading
			case "rendering":
				j.Status = JobStatusRendering
			case "uploading":
				j.Status = JobStatusUploading
			case "complete":
				j.Status = JobStatusComplete
			}
		})

		// Report to client
		if w.reporter != nil {
			w.reporter(job.ID, percent, fps, speed, eta, stage)
		}
	}, 500*time.Millisecond)

	return func(percent float64, fps int, speed string, eta int, stage string) {
		reporter.Report(progress.Progress{
			Progress: percent,
			FPS:      float64(fps),
			Speed:    0, // Speed multiplier not used here
		}, stage)
	}
}
