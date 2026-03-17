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
	"github.com/prmichaelsen/cloudcut-media-server/internal/storage"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()

	gcs, err := storage.NewGCSClient(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to create GCS client: %v", err)
	}
	defer gcs.Close()

	router := api.NewRouter(gcs)

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
