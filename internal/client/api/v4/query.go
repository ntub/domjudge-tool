package v4

import (
	"net/url"
	"strings"
)

// buildQuery encodes vals into a URL query string prefixed with "?", or
// returns "" when vals is empty. Percent-encoded square brackets produced
// by url.Values.Encode are restored to their literal form so that
// multi-value parameters use the ids[]=a&ids[]=b notation that DOMjudge
// expects (matching the Python httpx encoding).
func buildQuery(vals url.Values) string {
	if len(vals) == 0 {
		return ""
	}
	s := vals.Encode()
	s = strings.NewReplacer("%5B", "[", "%5D", "]").Replace(s)
	return "?" + s
}
