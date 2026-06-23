package storages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	ErrValidation = errors.New("invalid input")
	ErrNotFound   = errors.New("storage not found")
	ErrForbidden  = errors.New("access denied")
)

type Service struct {
	repo            *Repository
	fileStoragePath string
}

func NewService(repo *Repository, fileStoragePath string) *Service {
	return &Service{repo: repo, fileStoragePath: fileStoragePath}
}

func (s *Service) Create(ctx context.Context, ownerID int64, name, typ string, maxFileSizeMB int64, allowedExt []string) (Storage, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Storage{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if typ != "personal" && typ != "global" {
		return Storage{}, fmt.Errorf("%w: type must be 'personal' or 'global'", ErrValidation)
	}
	if maxFileSizeMB <= 0 {
		return Storage{}, fmt.Errorf("%w: max_file_size_mb must be greater than 0", ErrValidation)
	}

	maxBytes := maxFileSizeMB * 1024 * 1024
	return s.repo.Create(ctx, ownerID, name, typ, maxBytes, normalizeExtensions(allowedExt))
}

func (s *Service) GetByID(ctx context.Context, id, userID int64) (Storage, error) {
	st, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Storage{}, ErrNotFound
		}
		return Storage{}, err
	}

	if st.Type == "global" || st.OwnerID == userID {
		return st, nil
	}

	_, hasAccess, err := s.repo.GetUserPermission(ctx, id, userID)
	if err != nil {
		return Storage{}, err
	}
	if hasAccess {
		return st, nil
	}

	return Storage{}, ErrNotFound
}

func (s *Service) ListShared(ctx context.Context, userID int64) ([]SharedStorage, error) {
	return s.repo.ListSharedWithUser(ctx, userID)
}

func (s *Service) ListMy(ctx context.Context, ownerID int64) ([]Storage, error) {
	return s.repo.ListByOwner(ctx, ownerID)
}

func (s *Service) ListGlobal(ctx context.Context) ([]Storage, error) {
	return s.repo.ListGlobal(ctx)
}

func (s *Service) Update(ctx context.Context, id, userID int64, name string, maxFileSizeMB int64, allowedExt []string) (Storage, error) {
	st, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Storage{}, ErrNotFound
		}
		return Storage{}, err
	}
	if st.OwnerID != userID {
		return Storage{}, ErrForbidden
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return Storage{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if maxFileSizeMB <= 0 {
		return Storage{}, fmt.Errorf("%w: max_file_size_mb must be greater than 0", ErrValidation)
	}

	maxBytes := maxFileSizeMB * 1024 * 1024
	return s.repo.Update(ctx, id, name, maxBytes, normalizeExtensions(allowedExt))
}

func (s *Service) Delete(ctx context.Context, id, userID int64) error {
	st, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if st.OwnerID != userID {
		return ErrForbidden
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	dir := filepath.Join(s.fileStoragePath, "storages", strconv.FormatInt(id, 10))
	return os.RemoveAll(dir)
}

func normalizeExtensions(exts []string) []string {
	result := []string{}
	for _, e := range exts {
		e = strings.ToLower(strings.TrimSpace(e))
		e = strings.TrimPrefix(e, ".")
		if e != "" {
			result = append(result, e)
		}
	}
	return result
}
