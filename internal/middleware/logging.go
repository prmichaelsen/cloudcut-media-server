package middleware

import (
	"net/http"
	"time"

	"github.com/prmichaelsen/cloudcut-media-server/internal/logger"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	bytes      int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.bytes += len(b)
	return rw.ResponseWriter.Write(b)
}

func RequestLogging(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			log.Info("http_request", map[string]interface{}{
				"method":      r.Method,
				"path":        r.URL.Path,
				"status":      rw.statusCode,
				"duration_ms": duration.Milliseconds(),
				"bytes":       rw.bytes,
				"user_agent":  r.UserAgent(),
				"remote_addr": r.RemoteAddr,
			})
		})
	}
}
