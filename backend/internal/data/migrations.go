package data

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func SchemaReady(ctx context.Context, db *sql.DB) bool {
	var version int
	var dirty bool
	err := db.QueryRowContext(ctx, "SELECT version, dirty FROM pl_schema_migrations").Scan(&version, &dirty)
	return err == nil && version == 7 && !dirty
}

func Migrate(db *sql.DB) error {
	return MigrateNamespace(db, migrationFiles, "pl_schema_migrations")
}

func MigrateNamespace(db *sql.DB, files fs.FS, historyTable string) error {
	source, err := iofs.New(files, "migrations")
	if err != nil {
		return err
	}
	driver, err := migratemysql.WithInstance(db, &migratemysql.Config{
		MigrationsTable: historyTable,
	})
	if err != nil {
		return fmt.Errorf("initialize migration driver: %w", err)
	}
	runner, err := migrate.NewWithInstance("iofs", source, "mysql", driver)
	if err != nil {
		_ = driver.Close()
		return err
	}
	defer runner.Close()
	if err := runner.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
