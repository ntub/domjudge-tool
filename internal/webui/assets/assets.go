// Package assets embeds the web UI's static assets.
// Generated styles are committed so the binary is self-contained.
package assets

import "embed"

//go:embed tailwind.css daisyui.css webui.css datastar.min.js
var FS embed.FS
