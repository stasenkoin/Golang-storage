package permissions

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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

func toResponse(p Permission) PermissionResponse {
	return PermissionResponse{
		ID:         p.ID,
		StorageID:  p.StorageID,
		UserID:     p.UserID,
		Permission: p.Permission,
		CreatedAt:  p.CreatedAt,
	}
}

func (h *Handler) Grant(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := middleware.UserID(r.Context())
	storageID, err := idParam(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid storage id")
		return
	}

	var req GrantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	perm, err := h.service.Grant(r.Context(), storageID, ownerID, req.Email, req.Permission)
	if err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, toResponse(perm))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := middleware.UserID(r.Context())
	storageID, err := idParam(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid storage id")
		return
	}

	list, err := h.service.List(r.Context(), storageID, ownerID)
	if err != nil {
		writeError(w, err)
		return
	}

	out := make([]GrantedAccessResponse, 0, len(list))
	for _, g := range list {
		out = append(out, GrantedAccessResponse{
			ID:         g.ID,
			UserID:     g.UserID,
			UserEmail:  g.UserEmail,
			Permission: g.Permission,
			CreatedAt:  g.CreatedAt,
		})
	}
	response.JSON(w, http.StatusOK, out)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := middleware.UserID(r.Context())
	storageID, err := idParam(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid storage id")
		return
	}
	permissionID, err := idParam(r, "permissionId")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid permission id")
		return
	}

	var req UpdatePermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	perm, err := h.service.UpdateLevel(r.Context(), storageID, permissionID, ownerID, req.Permission)
	if err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toResponse(perm))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := middleware.UserID(r.Context())
	storageID, err := idParam(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid storage id")
		return
	}
	permissionID, err := idParam(r, "permissionId")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid permission id")
		return
	}

	if err := h.service.Revoke(r.Context(), storageID, permissionID, ownerID); err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func idParam(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, name), 10, 64)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		response.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrForbidden):
		response.Error(w, http.StatusForbidden, "access denied")
	case errors.Is(err, ErrAlreadyGranted):
		response.Error(w, http.StatusConflict, "user already has access")
	case errors.Is(err, ErrUserNotFound):
		response.Error(w, http.StatusNotFound, "user with this email not found")
	case errors.Is(err, ErrStorageNotFound):
		response.Error(w, http.StatusNotFound, "storage not found")
	case errors.Is(err, ErrPermissionNotFound):
		response.Error(w, http.StatusNotFound, "permission not found")
	default:
		response.Error(w, http.StatusInternalServerError, "internal error")
	}
}
