package intelligence

import "embed"

//go:embed migrations/*.sql
var schema embed.FS
