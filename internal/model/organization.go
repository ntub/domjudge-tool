package model

// Organization maps the DOMjudge v4 "organization" JSON shape. DOMjudge v7's
// web UI calls the same concept an affiliation; API-facing code keeps the
// API term.
type Organization struct {
	ID         string  `json:"id"`
	ICPCID     *string `json:"icpc_id"`
	ShortName  string  `json:"shortname"`
	Name       string  `json:"name"`
	FormalName *string `json:"formal_name"`
	Country    string  `json:"country"`
	URL        *string `json:"url"`
	Hashtag    *string `json:"twitter_hashtag"`
}
