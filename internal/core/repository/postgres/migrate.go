package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"filestorage/migrations"
)

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, migrations.InitSQL); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}
	return nil
}
