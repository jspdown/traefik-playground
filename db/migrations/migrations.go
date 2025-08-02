package migrations

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.sql
var migrationFS embed.FS

// Migrate migrates the database.
func Migrate(db *sql.DB) error {
	// Migrate the database.
	migrationSource, err := iofs.New(migrationFS, ".")
	if err != nil {
		return fmt.Errorf("reading migrations: %w", err)
	}
	defer func() { _ = migrationSource.Close() }()

	driver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: "migrations",
	})
	if err != nil {
		return fmt.Errorf("creating driver: %w", err)
	}

	migrator, err := migrate.NewWithInstance("iofs", migrationSource, "postgres", driver)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}

	if err = migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("up: %w", err)
	}

	return nil
}
