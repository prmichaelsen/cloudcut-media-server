package api

import (
	"encoding/json"
	"net/http"

	"github.com/prmichaelsen/cloudcut-media-server/internal/storage"
)

func NewRouter(gcs *storage.GCSClient) http.Handler {
	mux := http.NewServeMux()
	h := NewHandlers(gcs)

	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /api/v1/media/upload", h.HandleUpload)
	mux.HandleFunc("GET /api/v1/media/{id}", h.HandleGetMedia)
	mux.HandleFunc("GET /api/v1/media/{id}/url", h.HandleGetSignedURL)
	mux.HandleFunc("GET /api/v1/media/{id}/proxy/url", h.HandleGetProxyURL)

	var handler http.Handler = mux
	handler = LoggingMiddleware(handler)
	handler = CORSMiddleware(handler)
	handler = RecoveryMiddleware(handler)

	return handler
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
