package tables

import (
	"database/sql"
	"embed"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Migrate(db *sql.DB) error {
	return data.MigrateNamespace(db, migrationFiles, "pl_sheet_schema_migrations")
}
