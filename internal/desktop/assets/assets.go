// Package assets embeds the desktop app's static CSS files.
// The files are produced by the frontend build (just build-desktop-css)
// and committed so the binary is self-contained.
package assets

import "embed"

//go:embed tailwind.css daisyui.css desktop.css datastar.min.js
var FS embed.FS
