package permissions

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type Permission struct {
	ID         int64
	StorageID  int64
	UserID     int64
	Permission string
	CreatedAt  time.Time
}

type GrantedAccess struct {
	ID         int64
	UserID     int64
	UserEmail  string
	Permission string
	CreatedAt  time.Time
}

const permissionColumns = `id, storage_id, user_id, permission, created_at`

func scanPermission(row pgx.Row) (Permission, error) {
	var p Permission
	err := row.Scan(&p.ID, &p.StorageID, &p.UserID, &p.Permission, &p.CreatedAt)
	return p, err
}

func (r *Repository) GetUserIDByEmail(ctx context.Context, email string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&id)
	return id, err
}

func (r *Repository) ExistsForUser(ctx context.Context, storageID, userID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM storage_permissions WHERE storage_id = $1 AND user_id = $2)`,
		storageID, userID,
	).Scan(&exists)
	return exists, err
}

func (r *Repository) Create(ctx context.Context, storageID, userID int64, permission string) (Permission, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO storage_permissions (storage_id, user_id, permission)
		 VALUES ($1, $2, $3)
		 RETURNING `+permissionColumns,
		storageID, userID, permission,
	)
	return scanPermission(row)
}

func (r *Repository) GetByID(ctx context.Context, id int64) (Permission, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+permissionColumns+` FROM storage_permissions WHERE id = $1`, id)
	return scanPermission(row)
}

func (r *Repository) ListByStorage(ctx context.Context, storageID int64) ([]GrantedAccess, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.user_id, u.email, p.permission, p.created_at
		 FROM storage_permissions p
		 JOIN users u ON u.id = p.user_id
		 WHERE p.storage_id = $1
		 ORDER BY p.id`,
		storageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []GrantedAccess{}
	for rows.Next() {
		var g GrantedAccess
		if err := rows.Scan(&g.ID, &g.UserID, &g.UserEmail, &g.Permission, &g.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, g)
	}
	return list, rows.Err()
}

func (r *Repository) UpdateLevel(ctx context.Context, id int64, permission string) (Permission, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE storage_permissions SET permission = $2 WHERE id = $1 RETURNING `+permissionColumns,
		id, permission,
	)
	return scanPermission(row)
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM storage_permissions WHERE id = $1`, id)
	return err
}
