package tables

import (
	"context"
	"database/sql"
	"embed"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func SchemaReady(ctx context.Context, db *sql.DB) bool {
	var version int
	var dirty bool
	err := db.QueryRowContext(ctx, "SELECT version, dirty FROM pl_sheet_schema_migrations").Scan(&version, &dirty)
	return err == nil && version == 1 && !dirty
}

func Migrate(db *sql.DB) error {
	return data.MigrateNamespace(db, migrationFiles, "pl_sheet_schema_migrations")
}
