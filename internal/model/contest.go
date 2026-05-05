package model

// Contest maps the contest JSON object returned by the DOMjudge v4 API.
type Contest struct {
	ID                       string       `json:"id"`
	ExternalID               *string      `json:"externalid"`
	ShortName                string       `json:"shortname"`
	Name                     string       `json:"name"`
	FormalName               *string      `json:"formal_name"`
	StartTime                *string      `json:"start_time"`
	EndTime                  *string      `json:"end_time"`
	Duration                 *string      `json:"duration"`
	ScoreboardFreezeDuration *string      `json:"scoreboard_freeze_duration"`
	PenaltyTime              *int         `json:"penalty_time"`
	State                    ContestState `json:"state"`
}

// ContestState maps the nested contest state object used by contest and
// scoreboard API responses.
type ContestState struct {
	Started      *string `json:"started"`
	Ended        *string `json:"ended"`
	Frozen       *string `json:"frozen"`
	Thawed       *string `json:"thawed"`
	Finalized    *string `json:"finalized"`
	EndOfUpdates *string `json:"end_of_updates"`
}
