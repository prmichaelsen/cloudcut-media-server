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
	"github.com/prmichaelsen/cloudcut-media-server/internal/media"
	"github.com/prmichaelsen/cloudcut-media-server/internal/render"
	"github.com/prmichaelsen/cloudcut-media-server/internal/storage"
	"github.com/prmichaelsen/cloudcut-media-server/internal/ws"
)

func handleEDLSubmit(session *ws.Session, msg *ws.Message, handlers *api.Handlers, renderer *render.Renderer) {
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

	// Submit render job
	job, err := renderer.Submit(session.ID, parsedEDL)
	if err != nil {
		log.Printf("failed to submit render job: %v", err)
		errMsg, _ := ws.NewMessage(ws.TypeJobError, "", ws.ErrorPayload{
			Message: fmt.Sprintf("failed to submit job: %v", err),
		})
		session.Send(errMsg)
		return
	}

	ackMsg, _ := ws.NewMessage(ws.TypeEDLAck, "", map[string]string{
		"projectId": parsedEDL.ProjectID,
		"jobId":     job.ID,
	})
	session.Send(ackMsg)

	// Start rendering in background with progress updates
	go func() {
		progressCb := func(progress float64, fps int, speed string, eta int, stage string) {
			progressMsg, _ := ws.NewMessage(ws.TypeJobProgress, "", ws.ProgressPayload{
				JobID:   job.ID,
				Percent: progress,
				FPS:     fps,
				Speed:   speed,
				ETA:     eta,
				Stage:   stage,
			})
			session.Send(progressMsg)
		}

		if err := renderer.Render(context.Background(), job, progressCb); err != nil {
			log.Printf("render failed for job %s: %v", job.ID, err)
			errMsg, _ := ws.NewMessage(ws.TypeJobError, "", ws.ErrorPayload{
				JobID:   job.ID,
				Message: fmt.Sprintf("render failed: %v", err),
			})
			session.Send(errMsg)
			return
		}

		// TODO: Generate signed URL for output
		completeMsg, _ := ws.NewMessage(ws.TypeJobComplete, "", ws.CompletePayload{
			JobID: job.ID,
			URL:   fmt.Sprintf("/api/v1/jobs/%s/output", job.ID),
		})
		session.Send(completeMsg)
		log.Printf("render complete for job %s", job.ID)
	}()
}

func main() {
	cfg := config.Load()

	ctx := context.Background()

	gcs, err := storage.NewGCSClient(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to create GCS client: %v", err)
	}
	defer gcs.Close()

	proxy := media.NewProxyGenerator(gcs, cfg)
	handlers := api.NewHandlers(gcs, proxy)

	// Setup renderer
	ffmpegRenderer := render.NewFFmpegRenderer(cfg.FFmpegPath)
	jobStorage := render.NewMemoryJobStorage()
	renderer := render.NewRenderer(nil, ffmpegRenderer, jobStorage)

	wsSrv := ws.NewServer(func(session *ws.Session, msg *ws.Message) {
		log.Printf("ws message: session=%s type=%s", session.ID, msg.Type)

		switch msg.Type {
		case ws.TypeEDLSubmit:
			handleEDLSubmit(session, msg, handlers, renderer)
		case ws.TypePing:
			session.Send(&ws.Message{Type: ws.TypePong})
		default:
			log.Printf("unknown message type: %s", msg.Type)
		}
	})

	router := api.NewRouter(gcs, proxy, wsSrv, handlers)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("cloudcut-media-server starting on :%d (env=%s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	log.Println("server stopped")
}
