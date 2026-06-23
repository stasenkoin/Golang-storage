package storages

import (
	"encoding/json"
	"errors"
	"net/http"
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

type StorageResponse struct {
	ID                int64     `json:"id"`
	OwnerID           int64     `json:"owner_id"`
	Name              string    `json:"name"`
	Type              string    `json:"type"`
	MaxFileSizeBytes  int64     `json:"max_file_size_bytes"`
	AllowedExtensions []string  `json:"allowed_extensions"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func toResponse(s Storage) StorageResponse {
	return StorageResponse{
		ID:                s.ID,
		OwnerID:           s.OwnerID,
		Name:              s.Name,
		Type:              s.Type,
		MaxFileSizeBytes:  s.MaxFileSizeBytes,
		AllowedExtensions: s.AllowedExtensions,
		CreatedAt:         s.CreatedAt,
		UpdatedAt:         s.UpdatedAt,
	}
}

func toResponseList(list []Storage) []StorageResponse {
	out := make([]StorageResponse, 0, len(list))
	for _, s := range list {
		out = append(out, toResponse(s))
	}
	return out
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())

	var req CreateStorageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	st, err := h.service.Create(r.Context(), userID, req.Name, req.Type, req.MaxFileSizeMB, req.AllowedExtensions)
	if err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, toResponse(st))
}

func (h *Handler) ListMy(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	list, err := h.service.ListMy(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toResponseList(list))
}

func (h *Handler) ListGlobal(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.ListGlobal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toResponseList(list))
}

type SharedStorageResponse struct {
	StorageResponse
	Permission string `json:"permission"`
	OwnerEmail string `json:"owner_email"`
}

func (h *Handler) ListShared(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	list, err := h.service.ListShared(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}

	out := make([]SharedStorageResponse, 0, len(list))
	for _, ss := range list {
		out = append(out, SharedStorageResponse{
			StorageResponse: toResponse(ss.Storage),
			Permission:      ss.Permission,
			OwnerEmail:      ss.OwnerEmail,
		})
	}
	response.JSON(w, http.StatusOK, out)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	id, err := storageID(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid storage id")
		return
	}
	st, err := h.service.GetByID(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toResponse(st))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	id, err := storageID(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid storage id")
		return
	}

	var req UpdateStorageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	st, err := h.service.Update(r.Context(), id, userID, req.Name, req.MaxFileSizeMB, req.AllowedExtensions)
	if err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toResponse(st))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	id, err := storageID(r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid storage id")
		return
	}
	if err := h.service.Delete(r.Context(), id, userID); err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func storageID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		response.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrForbidden):
		response.Error(w, http.StatusForbidden, "access denied")
	case errors.Is(err, ErrNotFound):
		response.Error(w, http.StatusNotFound, "storage not found")
	default:
		response.Error(w, http.StatusInternalServerError, "internal error")
	}
}
