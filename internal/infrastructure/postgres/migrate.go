package postgres

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// MigrateUp runs all pending database migrations.
func MigrateUp(dsn, migrationsPath string) error {
	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		fmt.Sprintf("pgx5://%s", stripScheme(dsn)),
	)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}

func stripScheme(dsn string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if len(dsn) > len(prefix) && dsn[:len(prefix)] == prefix {
			return dsn[len(prefix):]
		}
	}
	return dsn
}
