package files

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"filestorage/internal/features/storages"
)

var (
	ErrValidation = errors.New("invalid input")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("access denied")
	ErrTooLarge   = errors.New("file is larger than the storage limit")
	ErrBadType    = errors.New("file extension is not allowed in this storage")
)

type Service struct {
	filesRepo       *Repository
	storagesRepo    *storages.Repository
	fileStoragePath string
}

func NewService(filesRepo *Repository, storagesRepo *storages.Repository, fileStoragePath string) *Service {
	return &Service{
		filesRepo:       filesRepo,
		storagesRepo:    storagesRepo,
		fileStoragePath: fileStoragePath,
	}
}

func (s *Service) Upload(ctx context.Context, storageID, userID int64, originalName string, size int64, contentType string, src io.Reader) (File, error) {
	st, err := s.storagesRepo.GetByID(ctx, storageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return File{}, ErrNotFound
		}
		return File{}, err
	}
	allowed, err := s.canUpload(ctx, st, userID)
	if err != nil {
		return File{}, err
	}
	if !allowed {
		return File{}, ErrForbidden
	}

	if size <= 0 {
		return File{}, fmt.Errorf("%w: file is empty", ErrValidation)
	}
	if size > st.MaxFileSizeBytes {
		return File{}, ErrTooLarge
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(originalName), "."))
	if !extensionAllowed(st.AllowedExtensions, ext) {
		return File{}, ErrBadType
	}

	storedName, err := randomName(ext)
	if err != nil {
		return File{}, err
	}

	relPath := filepath.Join("storages", strconv.FormatInt(storageID, 10), storedName)
	fullPath := filepath.Join(s.fileStoragePath, relPath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return File{}, err
	}

	dst, err := os.Create(fullPath)
	if err != nil {
		return File{}, err
	}
	written, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(fullPath)
		if copyErr != nil {
			return File{}, copyErr
		}
		return File{}, closeErr
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	file, err := s.filesRepo.Create(ctx, File{
		StorageID:    storageID,
		UploadedBy:   userID,
		OriginalName: originalName,
		StoredName:   storedName,
		DiskPath:     relPath,
		SizeBytes:    written,
		Extension:    ext,
		MimeType:     contentType,
	})
	if err != nil {
		os.Remove(fullPath)
		return File{}, err
	}
	return file, nil
}

func (s *Service) List(ctx context.Context, storageID, userID int64) ([]File, error) {
	st, err := s.storagesRepo.GetByID(ctx, storageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	allowed, err := s.canView(ctx, st, userID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrNotFound // чужое личное без доступа — прячем
	}
	return s.filesRepo.ListByStorage(ctx, storageID)
}

func (s *Service) Download(ctx context.Context, fileID, userID int64) (File, string, error) {
	f, err := s.filesRepo.GetByID(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return File{}, "", ErrNotFound
		}
		return File{}, "", err
	}
	st, err := s.storagesRepo.GetByID(ctx, f.StorageID)
	if err != nil {
		return File{}, "", err
	}
	allowed, err := s.canView(ctx, st, userID)
	if err != nil {
		return File{}, "", err
	}
	if !allowed {
		shared, err := s.filesRepo.HasSharedAccess(ctx, fileID, userID)
		if err != nil {
			return File{}, "", err
		}
		if !shared {
			return File{}, "", ErrNotFound
		}
	}
	fullPath := filepath.Join(s.fileStoragePath, f.DiskPath)
	return f, fullPath, nil
}

func (s *Service) Delete(ctx context.Context, fileID, userID int64) error {
	f, err := s.filesRepo.GetByID(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	st, err := s.storagesRepo.GetByID(ctx, f.StorageID)
	if err != nil {
		return err
	}
	allowed, err := s.canDelete(ctx, st, f, userID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	fullPath := filepath.Join(s.fileStoragePath, f.DiskPath)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return s.filesRepo.Delete(ctx, fileID)
}

func (s *Service) canDelete(ctx context.Context, st storages.Storage, f File, userID int64) (bool, error) {
	if st.OwnerID == userID {
		return true, nil
	}
	if f.UploadedBy != userID {
		return false, nil
	}
	if st.Type == "global" {
		return true, nil
	}
	level, hasAccess, err := s.storagesRepo.GetUserPermission(ctx, st.ID, userID)
	if err != nil {
		return false, err
	}
	return hasAccess && level == "write", nil
}

func (s *Service) canView(ctx context.Context, st storages.Storage, userID int64) (bool, error) {
	if st.Type == "global" || st.OwnerID == userID {
		return true, nil
	}
	_, hasAccess, err := s.storagesRepo.GetUserPermission(ctx, st.ID, userID)
	if err != nil {
		return false, err
	}
	return hasAccess, nil
}

func (s *Service) canUpload(ctx context.Context, st storages.Storage, userID int64) (bool, error) {
	if st.Type == "global" || st.OwnerID == userID {
		return true, nil
	}
	level, hasAccess, err := s.storagesRepo.GetUserPermission(ctx, st.ID, userID)
	if err != nil {
		return false, err
	}
	return hasAccess && level == "write", nil
}

func extensionAllowed(allowed []string, ext string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if a == ext {
			return true
		}
	}
	return false
}

func randomName(ext string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	name := hex.EncodeToString(b)
	if ext != "" {
		name = name + "." + ext
	}
	return name, nil
}
