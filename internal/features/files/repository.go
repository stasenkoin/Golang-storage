package files

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

type File struct {
	ID           int64
	StorageID    int64
	UploadedBy   int64
	OriginalName string
	StoredName   string
	DiskPath     string
	SizeBytes    int64
	Extension    string
	MimeType     string
	CreatedAt    time.Time
}

const fileColumns = `id, storage_id, uploaded_by, original_name, stored_name, disk_path, size_bytes, extension, mime_type, created_at`

func scanFile(row pgx.Row) (File, error) {
	var f File
	err := row.Scan(&f.ID, &f.StorageID, &f.UploadedBy, &f.OriginalName, &f.StoredName, &f.DiskPath, &f.SizeBytes, &f.Extension, &f.MimeType, &f.CreatedAt)
	return f, err
}

func (r *Repository) Create(ctx context.Context, f File) (File, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO files (storage_id, uploaded_by, original_name, stored_name, disk_path, size_bytes, extension, mime_type)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING `+fileColumns,
		f.StorageID, f.UploadedBy, f.OriginalName, f.StoredName, f.DiskPath, f.SizeBytes, f.Extension, f.MimeType,
	)
	return scanFile(row)
}

func (r *Repository) GetByID(ctx context.Context, id int64) (File, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+fileColumns+` FROM files WHERE id = $1`, id)
	return scanFile(row)
}

func (r *Repository) ListByStorage(ctx context.Context, storageID int64) ([]File, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+fileColumns+` FROM files WHERE storage_id = $1 ORDER BY id`,
		storageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []File{}
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	return list, rows.Err()
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM files WHERE id = $1`, id)
	return err
}

type SharedFile struct {
	File
	SharedByEmail string
	AccessedAt    time.Time
}

func (r *Repository) AddSharedAccess(ctx context.Context, fileID, userID, sharedBy, shareLinkID int64) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO shared_file_access (file_id, user_id, shared_by, share_link_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (file_id, user_id) DO NOTHING`,
		fileID, userID, sharedBy, shareLinkID,
	)
	return err
}

func (r *Repository) HasSharedAccess(ctx context.Context, fileID, userID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM shared_file_access WHERE file_id = $1 AND user_id = $2)`,
		fileID, userID,
	).Scan(&exists)
	return exists, err
}

func (r *Repository) DeleteSharedAccessByLink(ctx context.Context, shareLinkID int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM shared_file_access WHERE share_link_id = $1`, shareLinkID)
	return err
}

func (r *Repository) ListSharedWithUser(ctx context.Context, userID int64) ([]SharedFile, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT f.id, f.storage_id, f.uploaded_by, f.original_name, f.stored_name, f.disk_path,
		        f.size_bytes, f.extension, f.mime_type, f.created_at,
		        u.email, a.created_at
		 FROM shared_file_access a
		 JOIN files f ON f.id = a.file_id
		 JOIN users u ON u.id = a.shared_by
		 WHERE a.user_id = $1
		 ORDER BY a.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []SharedFile{}
	for rows.Next() {
		var sf SharedFile
		err := rows.Scan(
			&sf.ID, &sf.StorageID, &sf.UploadedBy, &sf.OriginalName, &sf.StoredName, &sf.DiskPath,
			&sf.SizeBytes, &sf.Extension, &sf.MimeType, &sf.CreatedAt,
			&sf.SharedByEmail, &sf.AccessedAt,
		)
		if err != nil {
			return nil, err
		}
		list = append(list, sf)
	}
	return list, rows.Err()
}
