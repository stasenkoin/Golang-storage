package auth

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type User struct {
	ID           int64
	Email        string
	PasswordHash string
}

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		email, passwordHash,
	).Scan(&id)
	return id, err
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, password_hash FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash)
	return u, err
}

func (r *Repository) GetUserByID(ctx context.Context, id int64) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, password_hash FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash)
	return u, err
}

func (r *Repository) EmailExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE email = $1)`,
		email,
	).Scan(&exists)
	return exists, err
}

func (r *Repository) SaveRefreshToken(ctx context.Context, token string, userID int64, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (token, user_id, expires_at) VALUES ($1, $2, $3)`,
		token, userID, expiresAt,
	)
	return err
}

type RefreshToken struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
}

func (r *Repository) GetRefreshToken(ctx context.Context, token string) (RefreshToken, error) {
	var rt RefreshToken
	err := r.pool.QueryRow(ctx,
		`SELECT token, user_id, expires_at FROM refresh_tokens WHERE token = $1`,
		token,
	).Scan(&rt.Token, &rt.UserID, &rt.ExpiresAt)
	return rt, err
}

func (r *Repository) DeleteRefreshToken(ctx context.Context, token string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE token = $1`,
		token,
	)
	return err
}
