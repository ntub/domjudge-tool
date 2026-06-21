package handler

import (
	"context"
	"net/http"

	"github.com/starfederation/datastar-go/datastar"
)

// DirectoryPickerFunc defines the signature for picking a directory path.
type DirectoryPickerFunc func(ctx context.Context) (string, error)

// RegisterPickDirectory registers the GET /api/pick-directory?signal=<name> endpoint.
func RegisterPickDirectory(mux *http.ServeMux, picker DirectoryPickerFunc) {
	mux.HandleFunc("GET /api/pick-directory", func(w http.ResponseWriter, r *http.Request) {
		signal := r.URL.Query().Get("signal")
		if signal == "" {
			signal = "outDir"
		}

		path, err := picker(r.Context())
		if err != nil || path == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		sse := datastar.NewSSE(w, r)
		_ = sse.MarshalAndPatchSignals(map[string]any{signal: path})
	})
}
