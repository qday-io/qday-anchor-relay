// Package migrations embeds the SQL migration files into the relay binary so
// image-based deploys need no external migrations directory — the relay
// applies them itself at startup (see store.Migrate).
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
