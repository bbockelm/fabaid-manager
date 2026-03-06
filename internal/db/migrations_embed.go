package db

import "embed"

// MigrationsFS contains the SQL migration files embedded at compile time.
// This allows the single binary to apply migrations without needing the
// migration files on disk.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
