package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"filestorage/internal/core/token"
)

var (
	ErrValidation         = errors.New("invalid input")
	ErrEmailTaken         = errors.New("email already taken")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidRefresh     = errors.New("invalid or expired refresh token")
)

type Service struct {
	repo            *Repository
	jwtSecret       string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewService(repo *Repository, jwtSecret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		repo:            repo,
		jwtSecret:       jwtSecret,
		accessTokenTTL:  accessTTL,
		refreshTokenTTL: refreshTTL,
	}
}

func (s *Service) Register(ctx context.Context, email, password string) (User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || len(password) < 6 {
		return User{}, ErrValidation
	}

	exists, err := s.repo.EmailExists(ctx, email)
	if err != nil {
		return User{}, err
	}
	if exists {
		return User{}, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}

	id, err := s.repo.CreateUser(ctx, email, string(hash))
	if err != nil {
		return User{}, err
	}
	return User{ID: id, Email: email}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (access string, refresh string, user User, err error) {
	email = strings.TrimSpace(strings.ToLower(email))

	user, err = s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", User{}, ErrInvalidCredentials
		}
		return "", "", User{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", "", User{}, ErrInvalidCredentials
	}

	access, err = token.NewAccessToken(user.ID, s.jwtSecret, s.accessTokenTTL)
	if err != nil {
		return "", "", User{}, err
	}

	refresh, err = s.issueRefreshToken(ctx, user.ID)
	if err != nil {
		return "", "", User{}, err
	}

	return access, refresh, user, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (access string, refresh string, err error) {
	rt, err := s.repo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrInvalidRefresh
		}
		return "", "", err
	}

	// токен истёк — удаляем запись и считаем невалидным
	if time.Now().After(rt.ExpiresAt) {
		_ = s.repo.DeleteRefreshToken(ctx, refreshToken)
		return "", "", ErrInvalidRefresh
	}

	access, err = token.NewAccessToken(rt.UserID, s.jwtSecret, s.accessTokenTTL)
	if err != nil {
		return "", "", err
	}

	if err := s.repo.DeleteRefreshToken(ctx, refreshToken); err != nil {
		return "", "", err
	}
	refresh, err = s.issueRefreshToken(ctx, rt.UserID)
	if err != nil {
		return "", "", err
	}

	return access, refresh, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.repo.DeleteRefreshToken(ctx, refreshToken)
}

func (s *Service) GetByID(ctx context.Context, id int64) (User, error) {
	return s.repo.GetUserByID(ctx, id)
}

func (s *Service) issueRefreshToken(ctx context.Context, userID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tokenStr := hex.EncodeToString(b)

	expiresAt := time.Now().Add(s.refreshTokenTTL)
	if err := s.repo.SaveRefreshToken(ctx, tokenStr, userID, expiresAt); err != nil {
		return "", err
	}
	return tokenStr, nil
}
