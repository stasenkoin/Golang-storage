package shares

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"

	"filestorage/internal/features/files"
	"filestorage/internal/features/storages"
)

var (
	ErrForbidden    = errors.New("access denied")
	ErrFileNotFound = errors.New("file not found")
	ErrLinkNotFound = errors.New("share link not found")
)

type Service struct {
	repo         *Repository
	filesRepo    *files.Repository
	storagesRepo *storages.Repository
}

func NewService(repo *Repository, filesRepo *files.Repository, storagesRepo *storages.Repository) *Service {
	return &Service{repo: repo, filesRepo: filesRepo, storagesRepo: storagesRepo}
}

func (s *Service) CreateLink(ctx context.Context, fileID, userID int64) (ShareLink, error) {
	f, err := s.filesRepo.GetByID(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ShareLink{}, ErrFileNotFound
		}
		return ShareLink{}, err
	}
	allowed, err := s.canShare(ctx, f, userID)
	if err != nil {
		return ShareLink{}, err
	}
	if !allowed {
		return ShareLink{}, ErrForbidden
	}

	token, err := randomToken()
	if err != nil {
		return ShareLink{}, err
	}
	return s.repo.Create(ctx, fileID, token, userID)
}

func (s *Service) ListLinks(ctx context.Context, fileID, userID int64) ([]ShareLink, error) {
	f, err := s.filesRepo.GetByID(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	allowed, err := s.canShare(ctx, f, userID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	return s.repo.ListActiveByFile(ctx, fileID)
}

func (s *Service) RevokeLink(ctx context.Context, linkID, userID int64) error {
	link, err := s.repo.GetByID(ctx, linkID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrLinkNotFound
		}
		return err
	}
	if link.CreatedBy != userID {
		return ErrForbidden
	}

	if err := s.repo.Deactivate(ctx, linkID); err != nil {
		return err
	}
	return s.filesRepo.DeleteSharedAccessByLink(ctx, linkID)
}

func (s *Service) OpenLink(ctx context.Context, token string, userID int64) (files.File, error) {
	link, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.File{}, ErrLinkNotFound
		}
		return files.File{}, err
	}
	if !link.IsActive {
		return files.File{}, ErrLinkNotFound
	}

	f, err := s.filesRepo.GetByID(ctx, link.FileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.File{}, ErrFileNotFound
		}
		return files.File{}, err
	}

	if userID != link.CreatedBy {
		if err := s.filesRepo.AddSharedAccess(ctx, f.ID, userID, link.CreatedBy, link.ID); err != nil {
			return files.File{}, err
		}
	}
	return f, nil
}

func (s *Service) ListSharedFiles(ctx context.Context, userID int64) ([]files.SharedFile, error) {
	return s.filesRepo.ListSharedWithUser(ctx, userID)
}

func (s *Service) canShare(ctx context.Context, f files.File, userID int64) (bool, error) {
	if f.UploadedBy == userID {
		return true, nil
	}
	st, err := s.storagesRepo.GetByID(ctx, f.StorageID)
	if err != nil {
		return false, err
	}
	return st.OwnerID == userID, nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
