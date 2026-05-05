// Package v7 implements the web.Gateway against DOMjudge 7.3.x's
// authenticated /jury HTML UI.
package v7

// Path constants mirror the Python enums at services/web/v7.py:
// HomePath, UserPath, TeamPath, AffiliationPath, ProblemPath. Format
// strings consume one ID via %s so callers don't sprintf inline.
const (
	pathLogin = "/login"
	pathJury  = "/jury"

	pathUsersList    = "/jury/users"
	pathUsersAdd     = "/jury/users/add"
	pathUsersEditFmt = "/jury/users/%s/edit"

	pathTeamsList    = "/jury/teams"
	pathTeamsAdd     = "/jury/teams/add"
	pathTeamsEditFmt = "/jury/teams/%s/edit"

	pathAffiliationsList = "/jury/affiliations"
	pathAffiliationsAdd  = "/jury/affiliations/add"

	pathProblemsList = "/jury/problems"
	pathProblemsAdd  = "/jury/problems/add"
)
