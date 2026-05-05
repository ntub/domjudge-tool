package importer

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

// defaultImportedGroupName is the placeholder name written into the
// optional group upsert when the operator passes --category-id but no
// matching group exists yet. The tradeoff is documented in PR-3 spec
// §4.3: the API needs a group row to attach teams to, and re-uploading
// is upsert-by-id so the placeholder is harmless on subsequent runs.
const defaultImportedGroupName = "p1.5-imported"

// resolveContestID converts a domserver.json contest_id hint into a
// concrete API contest id. Behaviour matches PR-3 SPEC §"邊界":
//
//   - non-empty hint: pass through Contests.Resolve (numeric hints
//     short-circuit; shortname hints hit /contests once).
//   - empty hint, one active contest: auto-select that contest's id —
//     preserves the v7 "server picks for me" UX for single-contest
//     installs.
//   - empty hint, zero or multiple active contests: explicit error
//     telling the operator to set contest_id in domserver.json.
//
// "Active" means the contest is started and not yet ended (state.started
// is set, state.ended is not). The behaviour change vs Phase 1 — which
// silently picked the server-default — is locked into the PR-5
// CHANGELOG "Behaviour changes (visible)" section.
func resolveContestID(ctx context.Context, api *apiv4.Client, hint string) (string, error) {
	if hint != "" {
		id, err := api.Contests.Resolve(ctx, hint)
		if err != nil {
			return "", fmt.Errorf("importer: resolve contest %q: %w", hint, err)
		}
		return id, nil
	}
	contests, err := api.Contests.List(ctx)
	if err != nil {
		return "", fmt.Errorf("importer: list contests: %w", err)
	}
	active := filterActiveContests(contests)
	switch len(active) {
	case 0:
		return "", fmt.Errorf("importer: cfg.contest_id is empty and no active contest exists; set contest_id in domserver.json")
	case 1:
		return active[0].ID, nil
	default:
		ids := make([]string, len(active))
		for i := range active {
			ids[i] = active[i].ID
		}
		return "", fmt.Errorf(
			"importer: cfg.contest_id is empty and %d active contests exist (%s); set contest_id in domserver.json",
			len(active), strings.Join(ids, ","),
		)
	}
}

// filterActiveContests returns the subset of contests that are
// currently in-flight: started ≤ now < ended. DOMjudge 7.3.x's
// /api/v4/contests list response only carries start_time / end_time
// (the nested state object is exposed at /contests/{cid}/state), so we
// derive activeness from those wall-clock fields when available and
// fall back to the legacy state predicate for responses (or alternate
// servers) that still carry it.
func filterActiveContests(contests []model.Contest) []model.Contest {
	now := time.Now()
	out := make([]model.Contest, 0, len(contests))
	for i := range contests {
		if isContestActive(contests[i], now) {
			out = append(out, contests[i])
		}
	}
	return out
}

func isContestActive(c model.Contest, now time.Time) bool {
	if c.StartTime != nil {
		return isActiveByTimes(c, now)
	}
	return isActiveByState(c)
}

func isActiveByTimes(c model.Contest, now time.Time) bool {
	start, ok := parseRFC3339(c.StartTime)
	if !ok || now.Before(start) {
		return false
	}
	end, hasEnd := parseRFC3339(c.EndTime)
	if hasEnd && !now.Before(end) {
		return false
	}
	return true
}

func isActiveByState(c model.Contest) bool {
	if c.State.Started == nil {
		return false
	}
	return c.State.Ended == nil
}

func parseRFC3339(s *string) (time.Time, bool) {
	if s == nil || *s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// apiCreateFlow drives the create-only path through DOMjudge's bulk
// import endpoints. It implements the algorithm in PR-3 §"API create
// flow 演算法":
//
//  1. resolve cid via resolveContestID.
//  2. if opts.CategoryID != nil, POST groups.tsv to ensure the team
//     category exists (idempotent upsert by id).
//  3. dedupe affiliations across rows; List existing organizations and
//     POST a JSON payload for the missing names.
//  4. POST teams2.tsv for every row.
//  5. POST accounts.tsv with every row's resolved password.
//  6. for every row with a non-empty CSV email, snapshot the user list
//     to look up the new user_id then call gw.UpdateUserEmail — the
//     CCS accounts.tsv format has no email column.
//
// Returns the input rows annotated with a derived password (from the
// CSV column or password.Generate when missing/forced) and IsExist=&false
// so the CLI's exporter.CreateUsers writes the same out.csv shape the
// scraper path produces.
func apiCreateFlow(
	ctx context.Context,
	api *apiv4.Client,
	gw web.Gateway,
	rows []model.CreateUser,
	contestHint string,
	opts ImportOptions,
) ([]model.CreateUser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cid, err := resolveContestID(ctx, api, contestHint)
	if err != nil {
		return nil, err
	}

	categoryID := derefIntOr(opts.CategoryID, 0)
	if opts.CategoryID != nil {
		groupTSV := formatGroupsTSV([]model.Group{{
			ID:   strconv.Itoa(categoryID),
			Name: defaultImportedGroupName,
		}})
		if _, err := api.Imports.GroupsTSV(ctx, strings.NewReader(groupTSV)); err != nil {
			return nil, fmt.Errorf("importer: import groups: %w", err)
		}
	}

	existingOrgs, err := api.Organizations.List(ctx, cid)
	if err != nil {
		return nil, fmt.Errorf("importer: list organizations: %w", err)
	}
	orgsByName := indexOrganizationsByName(existingOrgs)

	missing := dedupeMissingAffiliations(rows, orgsByName)
	if len(missing) > 0 {
		payload := buildOrganizationsPayload(missing, opts.AffiliationCountry)
		if _, err := api.Imports.Organizations(ctx, payload); err != nil {
			return nil, fmt.Errorf("importer: import organizations: %w", err)
		}
		// Merge the orgs we just created into the lookup map. The
		// payload already holds Name/ShortName/Country exactly as the
		// teams TSV writer wants, so we don't need a second round-trip
		// to /contests/{cid}/organizations to read back ids whose
		// values we already know.
		for _, org := range payload {
			orgsByName[org.Name] = org
		}
	}

	// Materialise per-row password before we render TSVs so the same
	// derived password ends up in accounts.tsv and the returned slice.
	out := make([]model.CreateUser, 0, len(rows))
	for _, row := range rows {
		pw, err := derivePassword(valueOrEmpty(row.Password), opts)
		if err != nil {
			return nil, fmt.Errorf("importer: derive password: %w", err)
		}
		row.Password = stringPtr(pw)
		isExist := false
		row.IsExist = &isExist
		out = append(out, row)
	}

	categoryIDStr := strconv.Itoa(categoryID)
	teamsTSV := formatTeamsTSV(out, categoryIDStr, orgsByName)
	if _, err := api.Imports.TeamsTSV(ctx, strings.NewReader(teamsTSV)); err != nil {
		return nil, fmt.Errorf("importer: import teams: %w", err)
	}

	accountsTSV := formatAccountsTSV(out)
	if _, err := api.Imports.Accounts(ctx, strings.NewReader(accountsTSV)); err != nil {
		return nil, fmt.Errorf("importer: import accounts: %w", err)
	}

	if hasEmailRow(out) {
		snap, err := api.Users.List(ctx, apiv4.UsersListOpts{})
		if err != nil {
			return nil, fmt.Errorf("importer: list users for email backfill: %w", err)
		}
		usersByName := indexByUsername(snap)
		for _, row := range out {
			if row.Email == nil || *row.Email == "" {
				continue
			}
			user, ok := usersByName[row.Username]
			if !ok {
				return nil, fmt.Errorf("importer: user %s not found in post-import snapshot", row.Username)
			}
			if err := gw.UpdateUserEmail(ctx, user.ID, *row.Email); err != nil {
				return nil, fmt.Errorf("importer: update email for %s: %w", row.Username, err)
			}
		}
	}

	return out, nil
}

// dedupeMissingAffiliations returns each unique CSV affiliation that is
// not already in orgsByName, in first-seen order so re-runs against the
// same input produce a stable POST payload.
//
// CSV parsers occasionally leave trailing whitespace (\r on the last
// column, NBSP, ordinary spaces) which makes a byte-exact lookup miss
// an existing org of the same logical name. v1.1.0 observed exactly
// this on a live v7 station — duplicate organization rows landed even
// though the dedup table looked correct. We strings.TrimSpace both the
// CSV column and the org name before comparing; the original
// (pre-trim) name is still what gets POSTed if it really is missing,
// to preserve operator intent. Debug-level logging dumps both sides
// when a row enters dedup so the next live run captures the data.
//
// TODO(phase-2): the trim is asymmetric — only the dedup comparison
// normalizes. formatTeamsTSV(tsv.go:54) still looks up orgs[raw],
// so a whitespace-bearing CSV row will dedup against existing orgs
// (good) but emit blank instName/instShortName/country in the team
// TSV (bad). Phase 2 reactivation should trim once at apiCreateFlow
// entry and propagate the trimmed value through every downstream
// lookup. v1.1.1 leaves it asymmetric because the path is dead code
// (router routes OpCreateTeamAndUser to scraper).
func dedupeMissingAffiliations(rows []model.CreateUser, orgsByName map[string]model.Organization) []string {
	trimmedKnown := make(map[string]struct{}, len(orgsByName))
	for name := range orgsByName {
		trimmedKnown[strings.TrimSpace(name)] = struct{}{}
	}
	seen := make(map[string]struct{}, len(rows))
	var out []string
	for _, row := range rows {
		if row.Affiliation == nil {
			continue
		}
		original := *row.Affiliation
		key := strings.TrimSpace(original)
		if key == "" {
			continue
		}
		slog.Debug("importer dedup affiliation",
			"csv_raw", original,
			"csv_bytes", []byte(original),
			"trimmed", key,
			"trimmed_bytes", []byte(key),
			"hit", contains(trimmedKnown, key),
		)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		if _, exists := trimmedKnown[key]; exists {
			continue
		}
		out = append(out, original)
	}
	return out
}

func contains(m map[string]struct{}, k string) bool {
	_, ok := m[k]
	return ok
}

// buildOrganizationsPayload turns missing affiliation names into the
// JSON shape POST /users/organizations expects. The id mirrors the
// shortname so re-uploads are upsert-by-id; the country is taken from
// opts.AffiliationCountry when non-empty.
func buildOrganizationsPayload(missing []string, country string) []model.Organization {
	out := make([]model.Organization, 0, len(missing))
	for _, name := range missing {
		out = append(out, model.Organization{
			ID:        name,
			ShortName: name,
			Name:      name,
			Country:   country,
		})
	}
	return out
}

// indexOrganizationsByName maps the contest's existing organizations by
// Name so the team TSV builder can look up institution metadata in O(1).
func indexOrganizationsByName(orgs []model.Organization) map[string]model.Organization {
	out := make(map[string]model.Organization, len(orgs))
	for i := range orgs {
		out[orgs[i].Name] = orgs[i]
	}
	return out
}

// hasEmailRow returns true when at least one row carries a non-empty
// email. The scraper-fallback path is not even attempted when no row
// needs it, so the post-import users.List call is also skipped.
func hasEmailRow(rows []model.CreateUser) bool {
	for _, row := range rows {
		if row.Email != nil && *row.Email != "" {
			return true
		}
	}
	return false
}
