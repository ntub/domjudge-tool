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

// defaultImportedGroupName names the placeholder group used by API-selected
// creation when a requested category has no matching group row.
const defaultImportedGroupName = "p1.5-imported"

// resolveContestID converts a domserver.json contest_id hint into a
// concrete API contest id:
//
//   - non-empty hint: pass through Contests.Resolve (numeric hints
//     short-circuit; shortname hints hit /contests once).
//   - empty hint, one active contest: auto-select that contest's id —
//     preserves the v7 "server picks for me" UX for single-contest
//     installs.
//   - empty hint, zero or multiple active contests: explicit error
//     telling the operator to set contest_id in domserver.json.
//
// "Active" means state.started is set and state.ended is not.
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

// apiCreateFlow sends an API-selected create cohort through bulk import,
// then backfills non-empty email through the gateway. The production default
// router does not select this path because account/team linkage is unsafe.
// Returned users include derived passwords and IsExist=false for CSV export.
func apiCreateFlow(
	ctx context.Context,
	api *apiv4.Client,
	gw web.Gateway,
	rows []model.CreateUser,
	contestHint string,
	opts ImportOptions,
) ([]model.CreateUser, error) {
	return apiCreateFlowWithSummary(ctx, api, gw, rows, contestHint, opts, nil)
}

func apiCreateFlowWithSummary(
	ctx context.Context,
	api *apiv4.Client,
	gw web.Gateway,
	rows []model.CreateUser,
	contestHint string,
	opts ImportOptions,
	summary *ImportUsersTeamsSummary,
) ([]model.CreateUser, error) {
	importRows := make([]importRow, 0, len(rows))
	for _, row := range rows {
		importRows = append(importRows, importRow{user: row})
	}
	return apiCreateFlowRowsWithSummary(ctx, api, gw, importRows, contestHint, opts, summary)
}

func apiCreateFlowRows(
	ctx context.Context,
	api *apiv4.Client,
	gw web.Gateway,
	rows []importRow,
	contestHint string,
	opts ImportOptions,
) ([]model.CreateUser, error) {
	return apiCreateFlowRowsWithSummary(ctx, api, gw, rows, contestHint, opts, nil)
}

func apiCreateFlowRowsWithSummary(
	ctx context.Context,
	api *apiv4.Client,
	gw web.Gateway,
	rows []importRow,
	contestHint string,
	opts ImportOptions,
	summary *ImportUsersTeamsSummary,
) ([]model.CreateUser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cid, err := resolveContestID(ctx, api, contestHint)
	if err != nil {
		return nil, err
	}

	categories := categoriesToUpsert(rows, opts.CategoryID)
	if len(categories) > 0 {
		groups := make([]model.Group, 0, len(categories))
		for _, categoryID := range categories {
			groups = append(groups, model.Group{
				ID:   strconv.Itoa(categoryID),
				Name: defaultImportedGroupName,
			})
		}
		groupTSV := formatGroupsTSV(groups)
		if _, err := api.Imports.GroupsTSV(ctx, strings.NewReader(groupTSV)); err != nil {
			return nil, fmt.Errorf("importer: import groups: %w", err)
		}
	}

	existingOrgs, err := api.Organizations.List(ctx, cid)
	if err != nil {
		return nil, fmt.Errorf("importer: list organizations: %w", err)
	}
	orgsByName := indexOrganizationsByName(existingOrgs)

	users := importUsers(rows)
	missing := dedupeMissingAffiliations(users, orgsByName)
	affiliationSummary := summarizeAPIAffiliations(users, orgsByName, missing)
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
	affiliationSummary.apply(summary)

	// Materialise per-row password before we render TSVs so the same
	// derived password ends up in accounts.tsv and the returned slice.
	out := make([]model.CreateUser, 0, len(rows))
	for _, row := range rows {
		pw, err := derivePassword(valueOrEmpty(row.user.Password), opts)
		if err != nil {
			return nil, fmt.Errorf("importer: derive password: %w", err)
		}
		user := row.user
		user.Password = stringPtr(pw)
		isExist := false
		user.IsExist = &isExist
		out = append(out, user)
	}

	teamsTSV := formatImportRowsTeamsTSV(rows, out, derefIntOr(opts.CategoryID, 0), orgsByName)
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

func importUsers(rows []importRow) []model.CreateUser {
	out := make([]model.CreateUser, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.user)
	}
	return out
}

func categoriesToUpsert(rows []importRow, fallback *int) []int {
	out := make([]int, 0, len(rows)+1)
	seen := make(map[int]struct{}, len(rows)+1)
	add := func(categoryID int) {
		if _, ok := seen[categoryID]; ok {
			return
		}
		seen[categoryID] = struct{}{}
		out = append(out, categoryID)
	}
	for _, row := range rows {
		if row.categoryID != nil {
			add(*row.categoryID)
		}
	}
	if fallback != nil {
		for _, row := range rows {
			if row.categoryID == nil {
				add(*fallback)
				break
			}
		}
	}
	if fallback != nil && len(rows) == 0 {
		add(*fallback)
	}
	return out
}

// dedupeMissingAffiliations returns missing names in first-seen order.
// Comparison trims surrounding whitespace, but POST payloads preserve the
// original names. The TSV organization lookup still uses raw names, so callers
// must not enable production API creation without reconciling normalization.
func dedupeMissingAffiliations(rows []model.CreateUser, orgsByName map[string]model.Organization) []string {
	trimmedKnown := make(map[string]struct{}, len(orgsByName))
	for name := range orgsByName {
		key := strings.TrimSpace(name)
		if key == "" {
			continue
		}
		trimmedKnown[key] = struct{}{}
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

type apiAffiliationSummary struct {
	createdNames []string
	reusedUnique int
	reusedRows   int
	fallbackRows int
}

func summarizeAPIAffiliations(
	rows []model.CreateUser,
	orgsByName map[string]model.Organization,
	missing []string,
) apiAffiliationSummary {
	trimmedKnown := make(map[string]struct{}, len(orgsByName))
	for name := range orgsByName {
		key := strings.TrimSpace(name)
		if key == "" {
			continue
		}
		trimmedKnown[key] = struct{}{}
	}

	out := apiAffiliationSummary{
		createdNames: append([]string(nil), missing...),
	}
	reusedSeen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row.Affiliation == nil {
			out.fallbackRows++
			continue
		}
		key := strings.TrimSpace(*row.Affiliation)
		if key == "" {
			out.fallbackRows++
			continue
		}
		if _, ok := trimmedKnown[key]; !ok {
			continue
		}
		out.reusedRows++
		if _, seen := reusedSeen[key]; seen {
			continue
		}
		reusedSeen[key] = struct{}{}
		out.reusedUnique++
	}
	return out
}

func (s apiAffiliationSummary) apply(summary *ImportUsersTeamsSummary) {
	if summary == nil {
		return
	}
	summary.AffiliationsCreated += len(s.createdNames)
	summary.AffiliationsCreatedNames = append(summary.AffiliationsCreatedNames, s.createdNames...)
	summary.AffiliationsReusedUnique += s.reusedUnique
	summary.AffiliationsReusedRows += s.reusedRows
	summary.FallbackAffiliationIDRows += s.fallbackRows
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
