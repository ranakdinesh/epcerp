// Package migrations publishes only Tender's immutable migration assets.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
