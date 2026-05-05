// Package scoreboard implements the public-scoreboard CSV exporter
// used by the `download-scoreboard` CLI. It is Layer-3 (above the API
// client, below CLI).
//
// Since Phase 1.5 the command uses the DOMjudge API JSON scoreboard and
// rebuilds the Phase 1 CSV shape in Build. The HTML parser remains in
// this package as a regression reference for the original Python output
// and possible public-mirror use cases.
package scoreboard
