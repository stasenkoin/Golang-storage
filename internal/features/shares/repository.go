package shares

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

type ShareLink struct {
	ID        int64
	FileID    int64
	Token     string
	CreatedBy int64
	IsActive  bool
	CreatedAt time.Time
}

const shareLinkColumns = `id, file_id, token, created_by, is_active, created_at`

func scanShareLink(row pgx.Row) (ShareLink, error) {
	var s ShareLink
	err := row.Scan(&s.ID, &s.FileID, &s.Token, &s.CreatedBy, &s.IsActive, &s.CreatedAt)
	return s, err
}

func (r *Repository) Create(ctx context.Context, fileID int64, token string, createdBy int64) (ShareLink, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO share_links (file_id, token, created_by)
		 VALUES ($1, $2, $3)
		 RETURNING `+shareLinkColumns,
		fileID, token, createdBy,
	)
	return scanShareLink(row)
}

func (r *Repository) GetByID(ctx context.Context, id int64) (ShareLink, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+shareLinkColumns+` FROM share_links WHERE id = $1`, id)
	return scanShareLink(row)
}

func (r *Repository) GetByToken(ctx context.Context, token string) (ShareLink, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+shareLinkColumns+` FROM share_links WHERE token = $1`, token)
	return scanShareLink(row)
}

func (r *Repository) ListActiveByFile(ctx context.Context, fileID int64) ([]ShareLink, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+shareLinkColumns+` FROM share_links WHERE file_id = $1 AND is_active = true ORDER BY id`,
		fileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []ShareLink{}
	for rows.Next() {
		s, err := scanShareLink(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *Repository) Deactivate(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE share_links SET is_active = false WHERE id = $1`, id)
	return err
}
