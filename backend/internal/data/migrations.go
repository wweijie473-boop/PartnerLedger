package data

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

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
	return err == nil && version == 2 && !dirty
}

func Migrate(db *sql.DB) error {
	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	driver, err := migratemysql.WithInstance(db, &migratemysql.Config{
		MigrationsTable: "pl_schema_migrations",
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
