package permissions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"filestorage/internal/features/storages"
)

var (
	ErrValidation         = errors.New("invalid input")
	ErrForbidden          = errors.New("access denied")
	ErrStorageNotFound    = errors.New("storage not found")
	ErrUserNotFound       = errors.New("user not found")
	ErrPermissionNotFound = errors.New("permission not found")
	ErrAlreadyGranted     = errors.New("user already has access")
)

type Service struct {
	repo         *Repository
	storagesRepo *storages.Repository
}

func NewService(repo *Repository, storagesRepo *storages.Repository) *Service {
	return &Service{repo: repo, storagesRepo: storagesRepo}
}

func (s *Service) Grant(ctx context.Context, storageID, ownerID int64, email, permission string) (Permission, error) {
	if permission != "read" && permission != "write" {
		return Permission{}, fmt.Errorf("%w: permission must be 'read' or 'write'", ErrValidation)
	}

	st, err := s.ownedPersonalStorage(ctx, storageID, ownerID)
	if err != nil {
		return Permission{}, err
	}

	email = strings.TrimSpace(strings.ToLower(email))
	targetID, err := s.repo.GetUserIDByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Permission{}, ErrUserNotFound
		}
		return Permission{}, err
	}
	if targetID == st.OwnerID {
		return Permission{}, fmt.Errorf("%w: cannot grant access to yourself", ErrValidation)
	}

	exists, err := s.repo.ExistsForUser(ctx, storageID, targetID)
	if err != nil {
		return Permission{}, err
	}
	if exists {
		return Permission{}, ErrAlreadyGranted
	}

	return s.repo.Create(ctx, storageID, targetID, permission)
}

func (s *Service) List(ctx context.Context, storageID, ownerID int64) ([]GrantedAccess, error) {
	if _, err := s.ownedPersonalStorage(ctx, storageID, ownerID); err != nil {
		return nil, err
	}
	return s.repo.ListByStorage(ctx, storageID)
}

func (s *Service) UpdateLevel(ctx context.Context, storageID, permissionID, ownerID int64, permission string) (Permission, error) {
	if permission != "read" && permission != "write" {
		return Permission{}, fmt.Errorf("%w: permission must be 'read' or 'write'", ErrValidation)
	}
	if _, err := s.ownedPersonalStorage(ctx, storageID, ownerID); err != nil {
		return Permission{}, err
	}

	perm, err := s.permissionOfStorage(ctx, permissionID, storageID)
	if err != nil {
		return Permission{}, err
	}
	if perm.Permission == permission {
		return perm, nil
	}
	return s.repo.UpdateLevel(ctx, permissionID, permission)
}

func (s *Service) Revoke(ctx context.Context, storageID, permissionID, ownerID int64) error {
	if _, err := s.ownedPersonalStorage(ctx, storageID, ownerID); err != nil {
		return err
	}
	if _, err := s.permissionOfStorage(ctx, permissionID, storageID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, permissionID)
}

func (s *Service) ownedPersonalStorage(ctx context.Context, storageID, ownerID int64) (storages.Storage, error) {
	st, err := s.storagesRepo.GetByID(ctx, storageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return storages.Storage{}, ErrStorageNotFound
		}
		return storages.Storage{}, err
	}
	if st.Type != "personal" {
		return storages.Storage{}, fmt.Errorf("%w: permissions are only for personal storages", ErrValidation)
	}
	if st.OwnerID != ownerID {
		return storages.Storage{}, ErrForbidden
	}
	return st, nil
}

func (s *Service) permissionOfStorage(ctx context.Context, permissionID, storageID int64) (Permission, error) {
	perm, err := s.repo.GetByID(ctx, permissionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Permission{}, ErrPermissionNotFound
		}
		return Permission{}, err
	}
	if perm.StorageID != storageID {
		return Permission{}, ErrPermissionNotFound
	}
	return perm, nil
}
