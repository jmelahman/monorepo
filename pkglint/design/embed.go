// Package design embeds the fonts and tokens pkglint's site shares with the
// monorepo's other frontends. Everything here but this file is synced from
// .config/git-orchard/shared/design in the monorepo; edit it there.
package design

import "embed"

//go:embed design.css fonts
var FS embed.FS
