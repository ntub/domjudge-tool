package web

import (
	"context"
	"io"

	"github.com/ntub/domjudge-tool/internal/model"
)

// Gateway is the version-agnostic contract for driving a DOMjudge
// jury HTML UI. v7.Client is the only Phase 1 implementation; Phase 2
// will add v8.Client beside it without changing this interface.
//
// Lifecycle: callers MUST call Login(ctx) before any other method.
// Phase 1 implementations do NOT auto-relogin on session expiry — a
// 302 to /login surfaces as ErrLoginFailed and the caller decides
// whether to retry.
//
// Mixed signature shapes (filter struct for problems vs. positional
// slices for delete ops) mirror the Python services/web/v7.py
// reference. They are intentionally NOT normalised; if a Phase-1+
// caller finds them painful we revisit at that point.
type Gateway interface {
	// Login establishes an authenticated session.
	Login(ctx context.Context) error

	// GetAffiliations returns one Affiliation per row of /jury/affiliations.
	GetAffiliations(ctx context.Context) ([]model.Affiliation, error)

	// GetAffiliation returns the first Affiliation whose Name or
	// ShortName matches name. Returns (nil, nil) on no match — matches
	// Python Optional[Affiliation].
	GetAffiliation(ctx context.Context, name string) (*model.Affiliation, error)

	// CreateAffiliation submits /jury/affiliations/add. The country
	// argument is required at this layer; the Python "TWN" default
	// belongs to importer / CLI callers.
	CreateAffiliation(ctx context.Context, shortname, name, country string) (*model.Affiliation, error)

	// GetProblems parses /jury/problems. opts is nil-safe (Python
	// __init__.py:21 panic regression locked).
	GetProblems(ctx context.Context, opts ProblemFilter) ([]model.ProblemItem, error)

	// SetUserPassword updates /jury/users/{userID}/edit. A nil roles
	// slice preserves the form's current roles; a non-nil slice
	// replaces them.
	//
	// Deprecated: prefer apiv4.ImportsResource since Phase 1.5; kept
	// for fallback when --user-roles is supplied (the API's
	// accounts.tsv has no role column) or when cfg.prefer_scraper is
	// true.
	SetUserPassword(ctx context.Context, userID, password string, roles []int, enabled bool) error

	// UpdateUserEmail updates /jury/users/{userID}/edit with a new email
	// address while preserving the rest of the form state.
	UpdateUserEmail(ctx context.Context, userID, email string) error

	// DeleteUsers iterates /jury/users and POSTs the delete link of
	// each row whose first-anchor text matches include and is not in
	// exclude. Locks the Python v7.py:161 inverted-filter quirk.
	DeleteUsers(ctx context.Context, include, exclude []string) error

	// CreateTeamAndUser submits /jury/teams/add and returns
	// (teamID, userID) parsed from the redirect path and the team
	// view's first .container-fluid anchor.
	//
	// Deprecated: prefer apiv4.ImportsResource since Phase 1.5; kept
	// for fallback when cfg.prefer_scraper is true or when the create
	// cohort is empty (the per-row scraper path runs only as the
	// router's fallback).
	CreateTeamAndUser(ctx context.Context, user model.CreateUser, categoryID, affiliationID int, enabled bool) (teamID, userID string, err error)

	// UpdateTeam submits /jury/teams/{user.TeamID}/edit and returns
	// (teamID, userID) the same way CreateTeamAndUser does.
	//
	// Deprecated: prefer apiv4.ImportsResource since Phase 1.5; kept
	// for fallback. Phase 1.5 keeps the update cohort on this method
	// because the live-station idempotency check for accounts.tsv /
	// teams2.tsv re-upload was not exercised on production data.
	UpdateTeam(ctx context.Context, user model.User, categoryID, affiliationID int, enabled bool) (teamID, userID string, err error)

	// DeleteTeams iterates /jury/teams. Same inverted-filter quirk
	// as DeleteUsers; the delete link is the second-to-last row
	// anchor (the last is the clarification link).
	DeleteTeams(ctx context.Context, include, exclude []string) error

	// Download performs an authenticated GET against the jury session
	// at path and returns the raw response body. The caller MUST close
	// the returned io.ReadCloser. Non-2xx responses surface as
	// implementation-specific *HTTPError values that match the package
	// sentinels (ErrUnauthorized / ErrServerError) via errors.Is.
	//
	// PR-7 added this so download-problems can stream export ZIPs
	// without callers having to type-assert to a concrete v7.Client.
	Download(ctx context.Context, path string) (io.ReadCloser, error)
}

// ProblemFilter mirrors Python services/web/v7.py:259-263 keyword
// args. Both fields are optional; nil/empty means "no filter".
type ProblemFilter struct {
	Only    []string
	Exclude []string
}
