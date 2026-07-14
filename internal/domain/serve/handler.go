package serve

import (
	"fmt"
	"log/slog"
	"net/http"
)

// handler builds the demo request multiplexer with the health and root
// endpoints. The HTTP lifecycle lives in gomatic/go-httpserver; this is the
// application-specific handler the template supplies to it.
func handler(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
	mux.HandleFunc("/", root(logger))
	return mux
}

// health responds to health checks.
func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, `{"status":"ok"}`)
}

// root responds to all other requests and logs the access.
func root(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info("Request received.", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "Example Server\n")
	}
}
