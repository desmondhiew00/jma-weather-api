// Package migrations embeds the SQL migration files so the worker binary and
// its schema cannot drift apart.
package migrations

import "embed"

// FS holds the migration files.
//
//go:embed *.sql
var FS embed.FS
