package storages

import (
	"context"
	"errors"
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

type Storage struct {
	ID                int64
	OwnerID           int64
	Name              string
	Type              string
	MaxFileSizeBytes  int64
	AllowedExtensions []string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type SharedStorage struct {
	Storage
	Permission string
	OwnerEmail string
}

const storageColumns = `id, owner_id, name, type, max_file_size_bytes, allowed_extensions, created_at, updated_at`

func scanStorage(row pgx.Row) (Storage, error) {
	var s Storage
	err := row.Scan(&s.ID, &s.OwnerID, &s.Name, &s.Type, &s.MaxFileSizeBytes, &s.AllowedExtensions, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

func (r *Repository) Create(ctx context.Context, ownerID int64, name, typ string, maxBytes int64, allowedExt []string) (Storage, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO storages (owner_id, name, type, max_file_size_bytes, allowed_extensions)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+storageColumns,
		ownerID, name, typ, maxBytes, allowedExt,
	)
	return scanStorage(row)
}

func (r *Repository) GetByID(ctx context.Context, id int64) (Storage, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+storageColumns+` FROM storages WHERE id = $1`, id)
	return scanStorage(row)
}

func (r *Repository) ListByOwner(ctx context.Context, ownerID int64) ([]Storage, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+storageColumns+` FROM storages WHERE owner_id = $1 AND type = 'personal' ORDER BY id`,
		ownerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStorages(rows)
}

func (r *Repository) ListGlobal(ctx context.Context) ([]Storage, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+storageColumns+` FROM storages WHERE type = 'global' ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStorages(rows)
}

func scanStorages(rows pgx.Rows) ([]Storage, error) {
	list := []Storage{}
	for rows.Next() {
		s, err := scanStorage(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *Repository) Update(ctx context.Context, id int64, name string, maxBytes int64, allowedExt []string) (Storage, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE storages
		 SET name = $2, max_file_size_bytes = $3, allowed_extensions = $4, updated_at = now()
		 WHERE id = $1
		 RETURNING `+storageColumns,
		id, name, maxBytes, allowedExt,
	)
	return scanStorage(row)
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM storages WHERE id = $1`, id)
	return err
}

func (r *Repository) GetUserPermission(ctx context.Context, storageID, userID int64) (string, bool, error) {
	var permission string
	err := r.pool.QueryRow(ctx,
		`SELECT permission FROM storage_permissions WHERE storage_id = $1 AND user_id = $2`,
		storageID, userID,
	).Scan(&permission)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return permission, true, nil
}

func (r *Repository) ListSharedWithUser(ctx context.Context, userID int64) ([]SharedStorage, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT s.id, s.owner_id, s.name, s.type, s.max_file_size_bytes, s.allowed_extensions, s.created_at, s.updated_at, p.permission, u.email
		 FROM storage_permissions p
		 JOIN storages s ON s.id = p.storage_id
		 JOIN users u ON u.id = s.owner_id
		 WHERE p.user_id = $1
		 ORDER BY s.id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []SharedStorage{}
	for rows.Next() {
		var ss SharedStorage
		err := rows.Scan(&ss.ID, &ss.OwnerID, &ss.Name, &ss.Type, &ss.MaxFileSizeBytes, &ss.AllowedExtensions, &ss.CreatedAt, &ss.UpdatedAt, &ss.Permission, &ss.OwnerEmail)
		if err != nil {
			return nil, err
		}
		list = append(list, ss)
	}
	return list, rows.Err()
}
