package importer

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ntub/domjudge-tool/internal/config"
)

// Op identifies an internal operation that may have both API and scraper
// implementations during the Phase 1.5 migration.
type Op int

const (
	OpCreateTeamAndUser Op = iota
	OpUpdateTeam
	OpSetPassword
	OpUpdateEmail
	OpDeleteUsers
	OpDeleteTeams
	OpResolveAffiliation
	OpListProblems
	OpDownloadProblemZip
	OpScoreboardSnapshot
)

// Path is the selected transport family for an operation.
type Path string

const (
	PathAPI     Path = "api"
	PathScraper Path = "scraper"
)

const (
	reasonPreferScraper            = "cfg.prefer_scraper=true"
	reasonAPILacksDelete           = "api lacks DELETE on v7"
	reasonAPILacksProblemZip       = "api lacks problem zip export on v7"
	reasonEmailNotInAccountsTSV    = "accounts.tsv has no email column"
	reasonRolesNotInAccountsTSV    = "accounts.tsv has no role column; explicit roles requested"
	reasonAccountsUpsertNotSafe    = "PR-0 finding: accounts.tsv re-upload is not idempotent"
	reasonAccountsTSVLinkageBroken = "accounts.tsv user→team linkage broken on v7"
	reasonDefaultAPIFirst          = "default api-first"
	affiliationResolutionGlobal    = false
)

// Context carries operation-specific facts used by the decision table.
type Context struct {
	Email string
	Roles []int
}

// Decision is the router's transport choice plus its stable reason string.
type Decision struct {
	Path   Path
	Reason string
}

// Router chooses the transport path for a hybrid operation.
type Router interface {
	Decide(ctx context.Context, op Op, in Context) Decision
}

// DefaultRouter implements the Phase 1.5 API-first decision table.
type DefaultRouter struct {
	cfg    *config.DomServerClient
	logger *slog.Logger
}

// RouterOption customizes a DefaultRouter.
type RouterOption func(*DefaultRouter)

// WithLogger routes telemetry to logger instead of slog.Default().
func WithLogger(logger *slog.Logger) RouterOption {
	return func(r *DefaultRouter) {
		r.logger = logger
	}
}

// NewDefaultRouter returns the production Phase 1.5 router.
func NewDefaultRouter(cfg *config.DomServerClient, opts ...RouterOption) *DefaultRouter {
	r := &DefaultRouter{
		cfg:    cfg,
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(r)
	}
	if r.logger == nil {
		r.logger = slog.Default()
	}
	return r
}

// Decide returns the first matching decision-table entry and emits one
// telemetry line for the choice.
func (r *DefaultRouter) Decide(ctx context.Context, op Op, in Context) Decision {
	_ = ctx
	d := decide(r.cfg, op, in)
	logDecision(r.logger, op, d.Path, d.Reason)
	return d
}

func decide(cfg *config.DomServerClient, op Op, in Context) Decision {
	if op == OpSetPassword && !accountsTSVUpsertSafe(cfg) {
		return Decision{Path: PathScraper, Reason: reasonAccountsUpsertNotSafe}
	}
	if preferScraper(cfg) && !apiOnly(op) {
		return Decision{Path: PathScraper, Reason: reasonPreferScraper}
	}
	switch op {
	case OpCreateTeamAndUser:
		// v1.1.1 revert: DOMjudge 7.3.x links accounts.tsv users to teams
		// via a username regex + Team::find primary-key lookup, with no
		// team-id column in the spec. The bulk import path silently
		// connects new users to small-id existing teams (admin/Example).
		// Phase 2 may reactivate this once the linkage quirk is fixed.
		return Decision{Path: PathScraper, Reason: reasonAccountsTSVLinkageBroken}
	case OpDeleteUsers, OpDeleteTeams:
		return Decision{Path: PathScraper, Reason: reasonAPILacksDelete}
	case OpDownloadProblemZip:
		return Decision{Path: PathScraper, Reason: reasonAPILacksProblemZip}
	case OpUpdateEmail:
		if in.Email != "" {
			return Decision{Path: PathScraper, Reason: reasonEmailNotInAccountsTSV}
		}
	case OpSetPassword:
		if in.Roles != nil {
			return Decision{Path: PathScraper, Reason: reasonRolesNotInAccountsTSV}
		}
	case OpResolveAffiliation:
		if affiliationResolutionGlobal {
			return Decision{Path: PathScraper, Reason: "organization API is global, scraper keeps contest scope"}
		}
	}
	return Decision{Path: PathAPI, Reason: reasonDefaultAPIFirst}
}

// accountsTSVUpsertSafe reports whether the bulk accounts.tsv re-upload path is
// safe for password resets. PR-0 confirmed it is idempotent on DOMjudge 7.3.x,
// but on 8.3.1 the bulk/PUT write path is broken (live-verified: PUT users/{id}
// 400 "already exists"; email dropped), so set-password must use the scraper.
func accountsTSVUpsertSafe(cfg *config.DomServerClient) bool {
	return !isV8Config(cfg)
}

func isV8Config(cfg *config.DomServerClient) bool {
	if cfg == nil {
		return false
	}
	return cfg.Version == "8.3" || strings.HasPrefix(cfg.Version, "8.3.")
}

func preferScraper(cfg *config.DomServerClient) bool {
	return cfg != nil && cfg.PreferScraper != nil && *cfg.PreferScraper
}

func apiOnly(op Op) bool {
	switch op {
	case OpListProblems, OpScoreboardSnapshot:
		return true
	default:
		return false
	}
}

func (op Op) String() string {
	switch op {
	case OpCreateTeamAndUser:
		return "create-team-and-user"
	case OpUpdateTeam:
		return "update-team"
	case OpSetPassword:
		return "set-password"
	case OpUpdateEmail:
		return "update-email"
	case OpDeleteUsers:
		return "delete-users"
	case OpDeleteTeams:
		return "delete-teams"
	case OpResolveAffiliation:
		return "resolve-affiliation"
	case OpListProblems:
		return "list-problems"
	case OpDownloadProblemZip:
		return "download-problem-zip"
	case OpScoreboardSnapshot:
		return "scoreboard-snapshot"
	default:
		return "unknown"
	}
}
