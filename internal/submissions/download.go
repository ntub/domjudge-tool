package submissions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// Download fetches every submission's source files for contest cid
// and writes one zip per submission under
// `<OutputPath>/<layout>/<judgement>_<id>.zip`. Layout is chosen by
// opts.Mode (see Options). Returns the count of submissions for
// which a zip was successfully written.
//
// Aborts on the first error from any underlying API call (matches
// Python: no per-row try/except). Submissions whose team_id or
// problem_id are not in the contest's index are skipped silently
// — Python parity, locks the user-base behaviour.
func Download(ctx context.Context, api *apiv4.Client, cid string, opts Options) (int, error) {
	teamsIdx, err := indexTeams(ctx, api, cid)
	if err != nil {
		return 0, err
	}
	problemsIdx, err := indexProblems(ctx, api, cid)
	if err != nil {
		return 0, err
	}
	judgements, err := buildJudgementMap(ctx, api, cid)
	if err != nil {
		return 0, err
	}

	listOpts := apiv4.SubmissionsListOpts{}
	if opts.Strict != nil {
		listOpts.Strict = opts.Strict
	}
	subs, err := api.Submissions.List(ctx, cid, listOpts)
	if err != nil {
		return 0, fmt.Errorf("submissions: list: %w", err)
	}

	rep := opts.Reporter
	if rep == nil {
		rep = progress.Nop()
	}
	rep.Start(len(subs), "downloading")
	defer rep.Finish()

	count := 0
	for i := range subs {
		sub := &subs[i]
		if err := ctx.Err(); err != nil {
			return count, fmt.Errorf("submissions: %w", err)
		}
		team, ok := teamsIdx[sub.TeamID]
		if !ok {
			continue
		}
		prob, ok := problemsIdx[sub.ProblemID]
		if !ok {
			continue
		}

		judgement := judgements[sub.ID]
		if judgement == "" {
			// No matching judgement → use the literal "none". Python
			// passes the dict.get() return (None) into an f-string
			// which serialises as "None"; we lock the same name with
			// a more explicit token so admins can grep their output.
			judgement = "none"
		}

		folder := buildFolder(opts.OutputPath, opts.Mode, cid, team, prob)
		if err := os.MkdirAll(folder, 0o755); err != nil {
			return count, fmt.Errorf("submissions: mkdir %s: %w", folder, err)
		}
		savedRaw, err := api.Submissions.Files(ctx, cid, sub.ID, folder)
		if err != nil {
			return count, fmt.Errorf("submissions: download %s: %w", sub.ID, err)
		}
		finalZip := filepath.Join(folder, fmt.Sprintf("%s_%s.zip", judgement, sub.ID))
		if savedRaw != finalZip {
			if err := os.Rename(savedRaw, finalZip); err != nil {
				return count, fmt.Errorf("submissions: rename %s → %s: %w", savedRaw, finalZip, err)
			}
		}
		if opts.Extract {
			if _, err := ExtractSingleFile(finalZip, fmt.Sprintf("%s_%s", judgement, sub.ID)); err != nil {
				return count, fmt.Errorf("submissions: extract %s: %w", sub.ID, err)
			}
		}
		count++
		rep.Tick()
	}
	return count, nil
}

// indexTeams fetches the contest's team list and indexes by ID for
// O(1) team_id → Team lookup during the per-submission loop.
func indexTeams(ctx context.Context, api *apiv4.Client, cid string) (map[string]model.Team, error) {
	teams, err := api.Teams.List(ctx, cid)
	if err != nil {
		return nil, fmt.Errorf("submissions: teams: %w", err)
	}
	out := make(map[string]model.Team, len(teams))
	for _, t := range teams {
		out[t.ID] = t
	}
	return out, nil
}

// indexProblems is the same shape as indexTeams but for Problem.
func indexProblems(ctx context.Context, api *apiv4.Client, cid string) (map[string]model.Problem, error) {
	probs, err := api.Problems.List(ctx, cid)
	if err != nil {
		return nil, fmt.Errorf("submissions: problems: %w", err)
	}
	out := make(map[string]model.Problem, len(probs))
	for _, p := range probs {
		out[p.ID] = p
	}
	return out, nil
}

// buildJudgementMap returns a map of submission_id → snake_case
// lowercase judgement_type name (e.g. "Accepted" → "accepted",
// "Wrong Answer" → "wrong_answer"). Mirrors Python's
// `judgement_submission_mapping` step inside `_submissions.py`.
func buildJudgementMap(ctx context.Context, api *apiv4.Client, cid string) (map[string]string, error) {
	types, err := api.JudgementTypes.List(ctx, cid, apiv4.JudgementTypesListOpts{})
	if err != nil {
		return nil, fmt.Errorf("submissions: judgement-types: %w", err)
	}
	typeName := make(map[string]string, len(types))
	for _, t := range types {
		typeName[t.ID] = strings.ToLower(strings.ReplaceAll(t.Name, " ", "_"))
	}
	jdgs, err := api.Judgements.List(ctx, cid, apiv4.JudgementsListOpts{})
	if err != nil {
		return nil, fmt.Errorf("submissions: judgements: %w", err)
	}
	out := make(map[string]string, len(jdgs))
	for _, j := range jdgs {
		if j.JudgementTypeID == nil {
			continue
		}
		out[j.SubmissionID] = typeName[*j.JudgementTypeID]
	}
	return out, nil
}

// buildFolder maps Mode + team/problem identity to the per-submission
// folder. The empty prefix case returns a relative path; OutputPath
// is unconditionally prepended when non-empty.
func buildFolder(prefix string, mode int, cid string, team model.Team, prob model.Problem) string {
	var rel string
	switch mode {
	case 1:
		rel = filepath.Join("team_"+team.Name, "problem_"+prob.ShortName)
	case 2:
		rel = filepath.Join("problem_"+prob.ShortName, "team_"+team.Name)
	default:
		rel = "contest_" + cid
	}
	if prefix == "" {
		return rel
	}
	return filepath.Join(prefix, rel)
}
