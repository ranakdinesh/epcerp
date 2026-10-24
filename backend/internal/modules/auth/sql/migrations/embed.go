// Package migrations publishes only Auth's immutable migration assets.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
