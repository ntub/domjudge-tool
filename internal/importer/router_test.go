package importer

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/ntub/domjudge-tool/internal/config"
)

func TestDefaultRouter_DecisionTable(t *testing.T) {
	trueValue := true
	cases := []struct {
		name       string
		cfg        *config.DomServerClient
		op         Op
		ctx        Context
		wantPath   Path
		wantReason string
	}{
		{
			name:       "OpCreateTeamAndUser_AlwaysScraperOnV7",
			op:         OpCreateTeamAndUser,
			wantPath:   PathScraper,
			wantReason: reasonAccountsTSVLinkageBroken,
		},
		{
			name:       "OpDeleteUsers_AlwaysScraper",
			op:         OpDeleteUsers,
			wantPath:   PathScraper,
			wantReason: reasonAPILacksDelete,
		},
		{
			name:       "OpDeleteTeams_AlwaysScraper",
			op:         OpDeleteTeams,
			wantPath:   PathScraper,
			wantReason: reasonAPILacksDelete,
		},
		{
			name:       "OpDownloadProblemZip_AlwaysScraper",
			op:         OpDownloadProblemZip,
			wantPath:   PathScraper,
			wantReason: reasonAPILacksProblemZip,
		},
		{
			name:       "OpUpdateEmail_WithEmail_Scraper",
			op:         OpUpdateEmail,
			ctx:        Context{Email: "alice@example.com"},
			wantPath:   PathScraper,
			wantReason: reasonEmailNotInAccountsTSV,
		},
		{
			name:       "OpUpdateEmail_NoEmail_API",
			op:         OpUpdateEmail,
			wantPath:   PathAPI,
			wantReason: reasonDefaultAPIFirst,
		},
		{
			name:       "OpSetPassword_NoRoles_API",
			op:         OpSetPassword,
			wantPath:   PathAPI,
			wantReason: reasonDefaultAPIFirst,
		},
		{
			name:       "OpSetPassword_WithRoles_Scraper",
			op:         OpSetPassword,
			ctx:        Context{Roles: []int{1, 2}},
			wantPath:   PathScraper,
			wantReason: reasonRolesNotInAccountsTSV,
		},
		{
			name:       "PreferScraperTrue_OverridesAPI",
			cfg:        &config.DomServerClient{PreferScraper: &trueValue},
			op:         OpSetPassword,
			wantPath:   PathScraper,
			wantReason: reasonPreferScraper,
		},
		{
			name:       "PreferScraperTrue_NoEffectOnAPIOnlyOp",
			cfg:        &config.DomServerClient{PreferScraper: &trueValue},
			op:         OpScoreboardSnapshot,
			wantPath:   PathAPI,
			wantReason: reasonDefaultAPIFirst,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			router := NewDefaultRouter(tc.cfg, WithLogger(testLogger(&buf)))
			got := router.Decide(context.Background(), tc.op, tc.ctx)
			if got.Path != tc.wantPath || got.Reason != tc.wantReason {
				t.Fatalf("Decision = %+v, want path=%q reason=%q", got, tc.wantPath, tc.wantReason)
			}
			out := buf.String()
			if !bytes.Contains([]byte(out), []byte("msg=\"router decision\"")) ||
				!bytes.Contains([]byte(out), []byte("op="+tc.op.String())) ||
				!bytes.Contains([]byte(out), []byte("path="+string(tc.wantPath))) ||
				!bytes.Contains([]byte(out), []byte("reason=")) {
				t.Fatalf("router telemetry missing decision attrs: %s", out)
			}
		})
	}
}

func testLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return attr
		},
	}))
}
