//go:build desktop

package main

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const desktopUserAgent = "domjudge-tool-desktop"

type downloadFilePicker func(context.Context, string) (io.WriteCloser, error)

// nativeDownloads keeps attachment POSTs on their form in WKWebView, which
// otherwise renders CSV as a new document instead of downloading it.
func nativeDownloads(next http.Handler, pick downloadFilePicker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.Contains(r.UserAgent(), desktopUserAgent) ||
			(r.URL.Path != "/export-users" && r.URL.Path != "/download-scoreboard") {
			next.ServeHTTP(w, r)
			return
		}
		d := &nativeDownloadWriter{ResponseWriter: w, ctx: r.Context(), pick: pick}
		defer func() {
			// Close on panic too; normal close errors are returned below.
			if d.file != nil {
				_ = d.file.Close()
			}
		}()
		next.ServeHTTP(d, r)
		if !d.started {
			return
		}
		if d.file != nil {
			if err := d.file.Close(); d.err == nil {
				d.err = err
			}
			d.file = nil
		}
		w.Header().Del("Content-Disposition")
		w.Header().Del("Content-Type")
		w.Header().Del("Content-Length")
		if d.err != nil {
			http.Error(w, fmt.Sprintf("save download: %v", d.err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

type nativeDownloadWriter struct {
	http.ResponseWriter
	ctx     context.Context
	pick    downloadFilePicker
	file    io.WriteCloser
	started bool
	err     error
}

func (d *nativeDownloadWriter) Write(body []byte) (int, error) {
	if !d.started {
		disposition := d.Header().Get("Content-Disposition")
		if disposition == "" {
			return d.ResponseWriter.Write(body)
		}
		d.started = true
		var params map[string]string
		_, params, d.err = mime.ParseMediaType(disposition)
		if d.err == nil {
			d.file, d.err = d.pick(d.ctx, params["filename"])
		}
	}
	if d.err != nil {
		return 0, d.err
	}
	if d.file == nil { // The user cancelled the picker.
		return len(body), nil
	}
	var n int
	n, d.err = d.file.Write(body)
	return n, d.err
}

func pickDownloadFile(ctx context.Context, app *application.App, name string) (io.WriteCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := app.Dialog.SaveFile().SetFilename(name).PromptForSingleSelection()
	if err != nil || path == "" {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open output: %w", err)
	}
	return file, nil
}
