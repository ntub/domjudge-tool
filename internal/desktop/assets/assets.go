// Package assets embeds the desktop app's static assets.
// Generated styles and the application icon are committed so the binary
// is self-contained.
package assets

import "embed"

//go:embed tailwind.css daisyui.css desktop.css datastar.min.js
var FS embed.FS

// Icon is the application's PNG icon, shared with the macOS bundle.
//
//go:embed appicon.png
var Icon []byte
