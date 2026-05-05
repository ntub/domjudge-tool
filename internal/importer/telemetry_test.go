package importer

import (
	"bytes"
	"log/slog"
	"testing"
)

func TestLogDecision_Format(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return attr
		},
	}))

	logDecision(logger, OpSetPassword, PathAPI, reasonDefaultAPIFirst)

	want := "level=INFO msg=\"router decision\" op=set-password path=api reason=\"default api-first\"\n"
	if got := buf.String(); got != want {
		t.Fatalf("log line mismatch\nwant: %q\n got: %q", want, got)
	}
}
