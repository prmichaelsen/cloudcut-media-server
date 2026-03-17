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
	"github.com/prmichaelsen/cloudcut-media-server/internal/storage"
	"github.com/prmichaelsen/cloudcut-media-server/internal/ws"
)

func handleEDLSubmit(session *ws.Session, msg *ws.Message, handlers *api.Handlers) {
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

	ackMsg, _ := ws.NewMessage(ws.TypeEDLAck, "", map[string]string{"projectId": parsedEDL.ProjectID})
	session.Send(ackMsg)

	// TODO: Task 6 - trigger rendering job
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

	wsSrv := ws.NewServer(func(session *ws.Session, msg *ws.Message) {
		log.Printf("ws message: session=%s type=%s", session.ID, msg.Type)

		switch msg.Type {
		case ws.TypeEDLSubmit:
			handleEDLSubmit(session, msg, handlers)
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
