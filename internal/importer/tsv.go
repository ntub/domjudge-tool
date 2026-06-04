package importer

import (
	"strconv"
	"strings"

	"github.com/ntub/domjudge-tool/internal/model"
)

// CCS bulk-import TSV format (DOMjudge 7 / contest-control-system spec):
//
//	groups.tsv:
//	  File_Version<TAB>1<LF>
//	  {id}<TAB>{name}<LF>
//	teams2.tsv:
//	  File_Version<TAB>2<LF>
//	  {teamID}<TAB>{externalID|""}<TAB>{categoryID}<TAB>
//	    {teamName}<TAB>{instName}<TAB>{instShortName}<TAB>
//	    {country|""}<TAB>{externalInstID|""}<LF>
//	accounts.tsv:
//	  accounts<TAB>1<LF>
//	  team<TAB>{fullName}<TAB>{username}<TAB>{password}<LF>
//
// All builders emit LF endings only — DOMjudge's import path rejects
// CRLF — and never prepend a UTF-8 BOM. Each function returns a string
// rather than driving an io.Writer so tests can compare against fixture
// bytes without intermediate buffering.

// formatGroupsTSV renders one row per group. An empty / nil slice still
// emits the File_Version header so the server receives a well-formed
// payload that imports zero groups.
func formatGroupsTSV(groups []model.Group) string {
	var b strings.Builder
	b.WriteString("File_Version\t1\n")
	for _, g := range groups {
		b.WriteString(g.ID)
		b.WriteByte('\t')
		b.WriteString(g.Name)
		b.WriteByte('\n')
	}
	return b.String()
}

// formatTeamsTSV renders one row per CreateUser. teamID is taken from
// the row's Username so re-uploads with the same CSV upsert by id.
// orgs maps the CSV affiliation column verbatim to a known organization;
// rows whose affiliation is nil or absent from the map degrade to blank
// instName / instShortName / country fields.
func formatTeamsTSV(rows []model.CreateUser, categoryID string, orgs map[string]model.Organization) string {
	importRows := make([]importRow, 0, len(rows))
	for _, row := range rows {
		importRows = append(importRows, importRow{user: row})
	}
	fallback, _ := strconv.Atoi(categoryID)
	return formatImportRowsTeamsTSV(importRows, rows, fallback, orgs)
}

func formatImportRowsTeamsTSV(importRows []importRow, rows []model.CreateUser, fallbackCategoryID int, orgs map[string]model.Organization) string {
	var b strings.Builder
	b.WriteString("File_Version\t2\n")
	for i, row := range rows {
		categoryID := fallbackCategoryID
		if i < len(importRows) && importRows[i].categoryID != nil {
			categoryID = *importRows[i].categoryID
		}
		var instName, instShortName, country string
		if row.Affiliation != nil {
			if org, ok := orgs[*row.Affiliation]; ok {
				instName = org.Name
				instShortName = org.ShortName
				country = org.Country
			}
		}
		b.WriteString(row.Username) // teamID
		b.WriteByte('\t')
		b.WriteByte('\t') // externalID — left blank
		b.WriteString(strconv.Itoa(categoryID))
		b.WriteByte('\t')
		b.WriteString(row.Name) // teamName
		b.WriteByte('\t')
		b.WriteString(instName)
		b.WriteByte('\t')
		b.WriteString(instShortName)
		b.WriteByte('\t')
		b.WriteString(country)
		b.WriteByte('\t')
		// externalInstID — left blank
		b.WriteByte('\n')
	}
	return b.String()
}

// formatAccountsTSV renders one `team` row per CreateUser. The password
// column comes from row.Password; nil prints as empty so callers can
// see the missing-password case in golden tests rather than panic.
func formatAccountsTSV(rows []model.CreateUser) string {
	var b strings.Builder
	b.WriteString("accounts\t1\n")
	for _, row := range rows {
		b.WriteString("team\t")
		b.WriteString(row.Name)
		b.WriteByte('\t')
		b.WriteString(row.Username)
		b.WriteByte('\t')
		if row.Password != nil {
			b.WriteString(*row.Password)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
