package importer

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/util/password"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// ImportOptions configures a single ImportUsersTeams call.
//
// Phase 1.5 added the API / Router / ContestHint fields so the
// importer can dispatch each row through the API path or the v7
// scraper path. CLI callers still talk to the same ImportUsersTeams
// entry point; the new fields are optional and default to a
// scraper-only flow when nil so existing tests keep working.
type ImportOptions struct {
	CategoryID         *int
	AffiliationID      *int
	UserRoles          []int
	AffiliationCountry string
	Enabled            bool
	IgnoreExisting     bool
	DeleteExisting     bool
	NewPassword        bool
	PasswordLength     int
	PasswordCharset    string

	// Router selects API vs scraper transport for each per-row op.
	// nil keeps the Phase 1 scraper-only behaviour.
	Router Router
	// API is the Phase 1.5 client used when Router elects PathAPI.
	// nil disables the API path even if Router asks for it.
	API *apiv4.Client
	// ContestHint is the cfg.contest_id value carried into the API
	// flow's resolveContestID. Empty triggers single-active-contest
	// auto-detection (PR-3 §"邊界").
	ContestHint string

	// Reporter is the optional progress sink. Library code never
	// constructs a Reporter; the CLI root PreRunE is the only
	// production construct site, and it passes the Reporter through
	// context so the call chain reaches this struct via the CLI
	// command. Nil is treated as [progress.Nop] so existing callers
	// (and Phase 1 tests that pre-date PR-9) need no change.
	Reporter progress.Reporter
}

// ImportUsersTeamsResult is the detailed return value for
// ImportUsersTeamsWithReport.
type ImportUsersTeamsResult struct {
	Users   []model.CreateUser
	Summary ImportUsersTeamsSummary
}

// ImportUsersTeamsSummary records the operator-facing outcome of one
// import-users-teams run.
type ImportUsersTeamsSummary struct {
	InputRows                 int
	ProcessedRows             int
	CreatedUsers              int
	UpdatedUsers              int
	SkippedExistingUsers      int
	AffiliationsCreated       int
	AffiliationsCreatedNames  []string
	AffiliationsReusedUnique  int
	AffiliationsReusedRows    int
	FallbackAffiliationIDRows int
	FallbackAffiliationID     int
}

// ImportUsersTeams reconciles a CSV against the live API user list and
// drives gw to create, update, or delete teams and users.
//
// Lifecycle: gw must NOT be pre-logged-in; ImportUsersTeams calls
// gw.Login(ctx) as its first gateway operation (mirrors Python
// context-manager "async with … as web: await web.login()").
//
// Returns []model.CreateUser with Password fields populated (either
// from the CSV column or freshly generated). The CLI layer is
// responsible for writing the output CSV from this slice using
// exporter.CreateUsers — this function never writes files.
//
// Per-row error policy: on the first error from any gateway call this
// function aborts and returns the slice accumulated so far together
// with the wrapped error, matching the Python reference (no per-row
// try/except).
func ImportUsersTeams(
	ctx context.Context,
	gw web.Gateway,
	users *apiv4.UsersResource,
	r io.Reader,
	opts ImportOptions,
) ([]model.CreateUser, error) {
	result, err := ImportUsersTeamsWithReport(ctx, gw, users, r, opts)
	return result.Users, err
}

// ImportUsersTeamsWithReport is ImportUsersTeams plus a structured
// summary suitable for CLI output.
func ImportUsersTeamsWithReport(
	ctx context.Context,
	gw web.Gateway,
	users *apiv4.UsersResource,
	r io.Reader,
	opts ImportOptions,
) (ImportUsersTeamsResult, error) {
	var result ImportUsersTeamsResult

	// Step 1 — login first so a stale or invalid session fails before
	// any state is changed.
	if err := gw.Login(ctx); err != nil {
		return result, fmt.Errorf("importer: login: %w", err)
	}

	// Step 2 — snapshot the current users so we can dispatch
	// create-vs-update.
	snapshot, err := users.List(ctx, apiv4.UsersListOpts{})
	if err != nil {
		return result, fmt.Errorf("importer: list users: %w", err)
	}

	// Step 3 — parse the CSV (BOM-tolerant, field-name driven).
	rows, err := readImportRows(r)
	if err != nil {
		return result, err
	}
	result.Summary.InputRows = len(rows)

	// Step 3.5 — start the optional progress bar over the parsed row
	// count. opts is taken by value, so writing back the Nop fallback
	// here is local to this call; the create / update sub-loops below
	// can call opts.Reporter.Tick() unconditionally without nil-checks.
	// One bar covers create + update phases; the split into two cohorts
	// is an internal optimisation, not part of the operator's mental
	// model.
	if opts.Reporter == nil {
		opts.Reporter = progress.Nop()
	}
	opts.Reporter.Start(len(rows), "importing")
	defer opts.Reporter.Finish()

	existingByName := indexByUsername(snapshot)

	// Step 4 — pre-delete pass when DeleteExisting is set. Must run
	// before the create/update pass so the dispatch below is not
	// surprised by half-deleted entries.
	if opts.DeleteExisting {
		if err := preDelete(ctx, gw, rows, existingByName); err != nil {
			return result, err
		}
	}

	// Resolve category-id / fallback affiliation-id from opts.
	categoryID := derefIntOr(opts.CategoryID, 0)
	fallbackAffID := derefIntOr(opts.AffiliationID, 0)
	result.Summary.FallbackAffiliationID = fallbackAffID
	affiliations := newAffiliationResolver(gw, opts.AffiliationCountry, &result.Summary)

	// Step 5 — split rows into create candidates vs update candidates.
	// DeleteExisting flips every row into a create candidate because
	// preDelete just removed the existing copy. IgnoreExisting drops
	// existing rows entirely.
	createRows := make([]importRow, 0, len(rows))
	type updateCandidate struct {
		row      importRow
		existing model.User
	}
	updateRows := make([]updateCandidate, 0, len(rows))
	for _, row := range rows {
		_, isExisting := existingByName[row.user.Username]
		if opts.DeleteExisting {
			isExisting = false
		}
		if isExisting {
			if opts.IgnoreExisting {
				result.Summary.SkippedExistingUsers++
				continue
			}
			updateRows = append(updateRows, updateCandidate{row: row, existing: existingByName[row.user.Username]})
			continue
		}
		createRows = append(createRows, row)
	}

	// Step 6 — route the create cohort. Phase 1.5 only migrates the
	// create-new path to the API; updates stay on the v7 scraper because
	// the live-station idempotency test for accounts/teams TSV upsert
	// hasn't shipped (PR-0 §"Findings"). When the router elects PathAPI
	// and an api client is wired, dispatch the whole cohort through one
	// apiCreateFlow call so the bulk endpoints see one TSV per kind.
	out := make([]model.CreateUser, 0, len(rows))
	created, err := processCreateCohort(ctx, gw, createRows, categoryID, fallbackAffID, opts, affiliations)
	result.Summary.CreatedUsers += len(created)
	result.Summary.ProcessedRows += len(created)
	result.Users = append(result.Users, created...)
	if err != nil {
		return result, err
	}
	out = append(out, created...)

	for i := range updateRows {
		// Phase 1.5 follow-up (PR-8): if the API list snapshot left
		// team_id == nil for an existing user, recover it via a two-
		// step lookup before handing the row to gw.UpdateTeam, which
		// rejects rows with no TeamID. Lazy: only fires on the rows
		// that actually need recovery.
		if updateRows[i].existing.TeamID == nil || *updateRows[i].existing.TeamID == "" {
			var teamsRes *apiv4.TeamsResource
			if opts.API != nil {
				teamsRes = opts.API.Teams
			}
			tid, err := resolveTeamID(ctx, users, teamsRes, opts.ContestHint, updateRows[i].existing)
			if err != nil {
				result.Users = out
				return result, fmt.Errorf("importer: update team %s: %w", updateRows[i].row.user.Username, err)
			}
			updateRows[i].existing.TeamID = &tid
		}

		rowCategoryID := effectiveCategoryID(updateRows[i].row, categoryID)
		processed, err := processRowScraper(ctx, gw, updateRows[i].row.user, true, updateRows[i].existing, rowCategoryID, fallbackAffID, opts, affiliations)
		if err != nil {
			result.Users = out
			return result, err
		}
		out = append(out, processed)
		result.Users = append(result.Users, processed)
		result.Summary.UpdatedUsers++
		result.Summary.ProcessedRows++
		opts.Reporter.Tick()
	}
	result.Users = out
	return result, nil
}

// processCreateCohort drives every row that needs a fresh team + user
// account. When the router elects PathAPI and opts.API is wired, the
// whole cohort goes through a single apiCreateFlow call (one TSV per
// import endpoint). Otherwise each row is processed individually
// against the v7 scraper, preserving Phase 1 semantics.
func processCreateCohort(
	ctx context.Context,
	gw web.Gateway,
	rows []importRow,
	categoryID, fallbackAffID int,
	opts ImportOptions,
	affiliations *affiliationResolver,
) ([]model.CreateUser, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	if useAPICreateFlow(ctx, opts) {
		var summary *ImportUsersTeamsSummary
		if affiliations != nil {
			summary = affiliations.summary
		}
		created, err := apiCreateFlowRowsWithSummary(ctx, opts.API, gw, rows, opts.ContestHint, opts, summary)
		if err != nil {
			return created, err
		}
		// API bulk path has no per-row loop — the bulk endpoints handle
		// the cohort in one shot. To keep the bar honest the cohort is
		// ticked en bloc; the bar jumps to its final count when the
		// bulk call returns, which the spec acknowledges as an accepted
		// trade-off (PR-9 §"7 條 batch 指令 wire-in" footnote).
		for range created {
			opts.Reporter.Tick()
		}
		return created, nil
	}
	out := make([]model.CreateUser, 0, len(rows))
	for _, row := range rows {
		rowCategoryID := effectiveCategoryID(row, categoryID)
		processed, err := processRowScraper(ctx, gw, row.user, false, model.User{}, rowCategoryID, fallbackAffID, opts, affiliations)
		if err != nil {
			return out, err
		}
		out = append(out, processed)
		opts.Reporter.Tick()
	}
	return out, nil
}

// useAPICreateFlow asks the router whether the create cohort should
// take the API path. Returns false when no router or api client is
// wired (Phase 1 callers without the new fields), so existing tests
// that pre-date PR-3 see no behaviour change.
func useAPICreateFlow(ctx context.Context, opts ImportOptions) bool {
	if opts.Router == nil || opts.API == nil {
		return false
	}
	d := opts.Router.Decide(ctx, OpCreateTeamAndUser, Context{})
	return d.Path == PathAPI
}

// processRowScraper runs the full per-row pipeline against the v7 web
// scraper: resolve affiliation, derive the password, dispatch update or
// create against the gateway, push the password through SetUserPassword,
// and finally sync the user email back into the DOMjudge user record
// when present. PR-3 added the API counterpart (apiCreateFlow) for the
// create cohort; this routine remains the scraper fallback and is
// always used for the update cohort.
func processRowScraper(
	ctx context.Context,
	gw web.Gateway,
	cu model.CreateUser,
	isExisting bool,
	existing model.User,
	categoryID, fallbackAffID int,
	opts ImportOptions,
	affiliations *affiliationResolver,
) (model.CreateUser, error) {
	var err error
	affID := fallbackAffID
	if cu.Affiliation != nil {
		name := strings.TrimSpace(*cu.Affiliation)
		if name == "" {
			cu.Affiliation = nil
			affiliations.recordFallback()
		} else {
			cu.Affiliation = &name
			affID, err = affiliations.resolve(ctx, name)
			if err != nil {
				return model.CreateUser{}, err
			}
		}
	} else {
		affiliations.recordFallback()
	}

	pw, err := derivePassword(valueOrEmpty(cu.Password), opts)
	if err != nil {
		return model.CreateUser{}, fmt.Errorf("importer: derive password: %w", err)
	}
	cu.Password = &pw
	cu.IsExist = &isExisting

	var userID string
	if isExisting {
		existing.Name = cu.Name // apply CSV name override before update
		// nolint:staticcheck // intentional scraper fallback (Phase 1.5 keeps updates here).
		_, userID, err = gw.UpdateTeam(ctx, existing, categoryID, affID, opts.Enabled)
		if err != nil {
			return model.CreateUser{}, fmt.Errorf("importer: update team %s: %w", cu.Username, err)
		}
	} else {
		// nolint:staticcheck // intentional scraper fallback when router elects PathScraper.
		_, userID, err = gw.CreateTeamAndUser(ctx, cu, categoryID, affID, opts.Enabled)
		if err != nil {
			return model.CreateUser{}, fmt.Errorf("importer: create team and user %s: %w", cu.Username, err)
		}
	}

	// nolint:staticcheck // intentional scraper fallback when router elects PathScraper.
	if err := gw.SetUserPassword(ctx, userID, pw, opts.UserRoles, opts.Enabled); err != nil {
		return model.CreateUser{}, fmt.Errorf("importer: set password for %s: %w", cu.Username, err)
	}
	if cu.Email != nil && *cu.Email != "" {
		if err := gw.UpdateUserEmail(ctx, userID, *cu.Email); err != nil {
			return model.CreateUser{}, fmt.Errorf("importer: update email for %s: %w", cu.Username, err)
		}
	}
	return cu, nil
}

// preDelete walks the parsed CSV rows, picks out the ones whose
// username exists in the snapshot, and drives the gateway to delete
// the corresponding users + teams. The skipped (not-in-snapshot) rows
// are left for the regular create path.
func preDelete(ctx context.Context, gw web.Gateway, rows []importRow, existing map[string]model.User) error {
	var deleteUsers, deleteTeams []string
	for _, row := range rows {
		username := row.user.Username
		ex, ok := existing[username]
		if !ok {
			continue
		}
		deleteUsers = append(deleteUsers, username)
		if ex.TeamID != nil {
			deleteTeams = append(deleteTeams, *ex.TeamID)
		}
	}
	if len(deleteUsers) == 0 {
		return nil
	}
	if err := gw.DeleteUsers(ctx, deleteUsers, nil); err != nil {
		return fmt.Errorf("importer: delete existing users: %w", err)
	}
	if err := gw.DeleteTeams(ctx, deleteTeams, nil); err != nil {
		return fmt.Errorf("importer: delete existing teams: %w", err)
	}
	return nil
}

func effectiveCategoryID(row importRow, fallback int) int {
	if row.categoryID != nil {
		return *row.categoryID
	}
	return fallback
}

// derivePassword returns the effective password for a row: the CSV
// column verbatim unless the column is empty or opts.NewPassword is
// set, in which case a fresh random password is generated. The default
// length / charset are pulled from the password package when opts
// values are zero.
func derivePassword(csvValue string, opts ImportOptions) (string, error) {
	if !opts.NewPassword && csvValue != "" {
		return csvValue, nil
	}
	charset := opts.PasswordCharset
	if charset == "" {
		charset = password.DefaultCharset
	}
	return password.Generate(opts.PasswordLength, charset)
}

func valueOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

type affiliationResolution int

const (
	affiliationResolutionCreated affiliationResolution = iota
	affiliationResolutionReused
)

type affiliationCacheEntry struct {
	id         int
	resolution affiliationResolution
}

type affiliationResolver struct {
	gw      web.Gateway
	country string
	cache   map[string]affiliationCacheEntry
	summary *ImportUsersTeamsSummary
}

func newAffiliationResolver(gw web.Gateway, country string, summary *ImportUsersTeamsSummary) *affiliationResolver {
	return &affiliationResolver{
		gw:      gw,
		country: country,
		cache:   make(map[string]affiliationCacheEntry),
		summary: summary,
	}
}

func (r *affiliationResolver) recordFallback() {
	if r == nil || r.summary == nil {
		return
	}
	r.summary.FallbackAffiliationIDRows++
}

func (r *affiliationResolver) resolve(ctx context.Context, name string) (int, error) {
	if r == nil {
		return 0, fmt.Errorf("importer: affiliation resolver unavailable")
	}
	if entry, ok := r.cache[name]; ok {
		r.recordCacheHit(entry)
		return entry.id, nil
	}

	aff, err := r.gw.GetAffiliation(ctx, name)
	if err != nil {
		return 0, fmt.Errorf("importer: get affiliation %q: %w", name, err)
	}
	resolution := affiliationResolutionReused
	if aff == nil {
		aff, err = r.gw.CreateAffiliation(ctx, name, name, r.country)
		if err != nil {
			return 0, fmt.Errorf("importer: create affiliation %q: %w", name, err)
		}
		resolution = affiliationResolutionCreated
	}
	if aff == nil || aff.ID == nil {
		return 0, fmt.Errorf("importer: affiliation %q has no id", name)
	}
	id, err := strconv.Atoi(*aff.ID)
	if err != nil {
		return 0, fmt.Errorf("importer: affiliation %q id %q is not numeric: %w", name, *aff.ID, err)
	}
	entry := affiliationCacheEntry{id: id, resolution: resolution}
	r.cache[name] = entry
	r.recordFirstResolution(name, entry)
	return id, nil
}

func (r *affiliationResolver) recordFirstResolution(name string, entry affiliationCacheEntry) {
	if r.summary == nil {
		return
	}
	switch entry.resolution {
	case affiliationResolutionCreated:
		r.summary.AffiliationsCreated++
		r.summary.AffiliationsCreatedNames = append(r.summary.AffiliationsCreatedNames, name)
	case affiliationResolutionReused:
		r.summary.AffiliationsReusedUnique++
		r.summary.AffiliationsReusedRows++
	}
}

func (r *affiliationResolver) recordCacheHit(entry affiliationCacheEntry) {
	if r == nil || r.summary == nil {
		return
	}
	if entry.resolution == affiliationResolutionReused {
		r.summary.AffiliationsReusedRows++
	}
}

// indexByUsername maps a snapshot of users by username so the
// create-vs-update dispatch is O(1) per row.
func indexByUsername(users []model.User) map[string]model.User {
	out := make(map[string]model.User, len(users))
	for i := range users {
		out[users[i].Username] = users[i]
	}
	return out
}

// derefIntOr returns *p when p is non-nil, otherwise the supplied
// fallback. The helper keeps the call sites in ImportUsersTeams free
// of nil-checks.
func derefIntOr(p *int, fallback int) int {
	if p == nil {
		return fallback
	}
	return *p
}
