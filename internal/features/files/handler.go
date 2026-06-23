package files

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"filestorage/internal/transport/http/middleware"
	"filestorage/internal/transport/http/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type FileResponse struct {
	ID           int64     `json:"id"`
	StorageID    int64     `json:"storage_id"`
	UploadedBy   int64     `json:"uploaded_by"`
	OriginalName string    `json:"original_name"`
	SizeBytes    int64     `json:"size_bytes"`
	Extension    string    `json:"extension"`
	MimeType     string    `json:"mime_type"`
	CreatedAt    time.Time `json:"created_at"`
}

func toResponse(f File) FileResponse {
	return FileResponse{
		ID:           f.ID,
		StorageID:    f.StorageID,
		UploadedBy:   f.UploadedBy,
		OriginalName: f.OriginalName,
		SizeBytes:    f.SizeBytes,
		Extension:    f.Extension,
		MimeType:     f.MimeType,
		CreatedAt:    f.CreatedAt,
	}
}

func toResponseList(list []File) []FileResponse {
	out := make([]FileResponse, 0, len(list))
	for _, f := range list {
		out = append(out, toResponse(f))
	}
	return out
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	storageID, err := idParam(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid storage id")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "file is required (form field 'file')")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	created, err := h.service.Upload(r.Context(), storageID, userID, header.Filename, header.Size, contentType, file)
	if err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, toResponse(created))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	storageID, err := idParam(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid storage id")
		return
	}
	list, err := h.service.List(r.Context(), storageID, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toResponseList(list))
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	fileID, err := idParam(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid file id")
		return
	}

	f, fullPath, err := h.service.Download(r.Context(), fileID, userID)
	if err != nil {
		writeError(w, err)
		return
	}

	src, err := os.Open(fullPath)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "file is missing on disk")
		return
	}
	defer src.Close()

	w.Header().Set("Content-Type", f.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, f.OriginalName))
	w.Header().Set("Content-Length", strconv.FormatInt(f.SizeBytes, 10))
	_, _ = io.Copy(w, src)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	fileID, err := idParam(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid file id")
		return
	}
	if err := h.service.Delete(r.Context(), fileID, userID); err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func idParam(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrValidation), errors.Is(err, ErrTooLarge), errors.Is(err, ErrBadType):
		response.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrForbidden):
		response.Error(w, http.StatusForbidden, "access denied")
	case errors.Is(err, ErrNotFound):
		response.Error(w, http.StatusNotFound, "not found")
	default:
		response.Error(w, http.StatusInternalServerError, "internal error")
	}
}
