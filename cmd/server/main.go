package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prmichaelsen/cloudcut-media-server/internal/api"
	"github.com/prmichaelsen/cloudcut-media-server/internal/config"
	"github.com/prmichaelsen/cloudcut-media-server/internal/edl"
	"github.com/prmichaelsen/cloudcut-media-server/internal/jobs"
	"github.com/prmichaelsen/cloudcut-media-server/internal/logger"
	"github.com/prmichaelsen/cloudcut-media-server/internal/media"
	"github.com/prmichaelsen/cloudcut-media-server/internal/middleware"
	"github.com/prmichaelsen/cloudcut-media-server/internal/render"
	"github.com/prmichaelsen/cloudcut-media-server/internal/storage"
	"github.com/prmichaelsen/cloudcut-media-server/internal/ws"
	"github.com/prmichaelsen/cloudcut-media-server/pkg/models"
)

func handleEDLSubmit(session *ws.Session, msg *ws.Message, handlers *api.Handlers, jobManager *jobs.JobManager, worker *jobs.Worker) {
	mediaExists := func(mediaID string) bool {
		_, ok := handlers.GetMedia(mediaID)
		return ok
	}

	parsedEDL, errs := edl.Parse(msg.Payload, mediaExists)
	if len(errs) > 0 {
		log.Printf("EDL validation failed: %v", errs)
		errMsg, _ := ws.NewMessage(ws.TypeJobError, "", ws.ErrorPayload{
			Message: errs.Error(),
		})
		session.Send(errMsg)
		return
	}

	log.Printf("EDL validated successfully: project=%s duration=%.2fs tracks=%d",
		parsedEDL.ProjectID, parsedEDL.Timeline.Duration, len(parsedEDL.Timeline.Tracks))

	// Create render job
	job := jobManager.CreateJob(jobs.JobTypeExportRender, session.ID, "", parsedEDL)

	ackMsg, _ := ws.NewMessage(ws.TypeEDLAck, "", map[string]string{
		"projectId": parsedEDL.ProjectID,
		"jobId":     job.ID,
	})
	session.Send(ackMsg)

	// Submit job to worker
	worker.Submit(job)
}

func main() {
	cfg := config.Load()
	appLogger := logger.New(cfg.Env)

	ctx := context.Background()

	gcs, err := storage.NewGCSClient(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to create GCS client: %v", err)
	}
	defer gcs.Close()

	proxy := media.NewProxyGenerator(gcs, cfg)

	// Setup renderer
	ffmpegRenderer := render.NewFFmpegRenderer(cfg.FFmpegPath)
	jobStorage := render.NewMemoryJobStorage()
	renderer := render.NewRenderer(gcs, ffmpegRenderer, jobStorage)

	// Setup job system
	jobManager := jobs.NewJobManager()

	// Progress reporter sends updates via WebSocket
	var wsSrv *ws.Server
	progressReporter := func(jobID string, percent float64, fps int, speed string, eta int, stage string) {
		// Find session for job
		job, ok := jobManager.GetJob(jobID)
		if !ok {
			return
		}

		if job.SessionID == "" {
			return
		}

		session, ok := wsSrv.GetSession(job.SessionID)
		if !ok {
			return
		}

		// Send progress update
		progressMsg, _ := ws.NewMessage(ws.TypeJobProgress, "", ws.ProgressPayload{
			JobID:   jobID,
			Percent: percent,
			FPS:     fps,
			Speed:   speed,
			ETA:     eta,
			Stage:   stage,
		})
		session.Send(progressMsg)

		// Send completion message
		if stage == "complete" {
			completeMsg, _ := ws.NewMessage(ws.TypeJobComplete, "", ws.CompletePayload{
				JobID: jobID,
				URL:   job.ResultURL,
			})
			session.Send(completeMsg)
		}
	}

	// Create handlers first (without worker)
	var worker *jobs.Worker
	handlers := api.NewHandlers(gcs, proxy, jobManager, worker)
	mediaStore := &mediaStoreAdapter{handlers: handlers}

	worker = jobs.NewWorker(jobManager, gcs, proxy, renderer, mediaStore, progressReporter, 2)
	worker.Start()

	// Update handlers with worker reference
	handlers.SetWorker(worker)

	wsSrv = ws.NewServer(func(session *ws.Session, msg *ws.Message) {
		appLogger.Debug("ws_message", map[string]interface{}{
			"session_id": session.ID,
			"type":       msg.Type,
		})

		switch msg.Type {
		case ws.TypeEDLSubmit:
			handleEDLSubmit(session, msg, handlers, jobManager, worker)
		case ws.TypePing:
			session.Send(&ws.Message{Type: ws.TypePong})
		default:
			appLogger.Warn("unknown_message_type", map[string]interface{}{
				"type": msg.Type,
			})
		}
	})

	router := api.NewRouter(gcs, proxy, wsSrv, handlers)

	// Wrap router with request logging middleware
	handler := middleware.RequestLogging(appLogger)(router)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		appLogger.Info("server_starting", map[string]interface{}{
			"port": cfg.Port,
			"env":  cfg.Env,
		})
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("server_shutting_down", nil)

	// Stop worker
	worker.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	appLogger.Info("server_stopped", nil)
}

// mediaStoreAdapter adapts Handlers to jobs.MediaStore interface.
type mediaStoreAdapter struct {
	handlers *api.Handlers
}

func (m *mediaStoreAdapter) GetMedia(mediaID string) (*models.Media, bool) {
	if m.handlers == nil {
		return nil, false
	}
	return m.handlers.GetMedia(mediaID)
}
