// Package assets holds images embedded in the binary.
package assets

import _ "embed"

// SteamLogo is the default picture for suspend and shutdown, cropped to
// the logo.
//
//go:embed steam-logo.png
var SteamLogo []byte
