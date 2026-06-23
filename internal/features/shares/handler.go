package shares

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"filestorage/internal/features/files"
	"filestorage/internal/transport/http/middleware"
	"filestorage/internal/transport/http/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	fileID, err := idParam(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid file id")
		return
	}
	link, err := h.service.CreateLink(r.Context(), fileID, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, linkResponse(link))
}

func (h *Handler) ListLinks(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	fileID, err := idParam(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid file id")
		return
	}
	links, err := h.service.ListLinks(r.Context(), fileID, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]ShareLinkResponse, 0, len(links))
	for _, l := range links {
		out = append(out, linkResponse(l))
	}
	response.JSON(w, http.StatusOK, out)
}

func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	linkID, err := idParam(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid share link id")
		return
	}
	if err := h.service.RevokeLink(r.Context(), linkID, userID); err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (h *Handler) Open(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	token := chi.URLParam(r, "token")
	f, err := h.service.OpenLink(r.Context(), token, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, sharedFileResponse(f))
}

func (h *Handler) ListSharedFiles(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserID(r.Context())
	list, err := h.service.ListSharedFiles(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]AvailableFileResponse, 0, len(list))
	for _, sf := range list {
		out = append(out, AvailableFileResponse{
			ID:            sf.ID,
			OriginalName:  sf.OriginalName,
			SizeBytes:     sf.SizeBytes,
			Extension:     sf.Extension,
			MimeType:      sf.MimeType,
			SharedByEmail: sf.SharedByEmail,
			AccessedAt:    sf.AccessedAt,
		})
	}
	response.JSON(w, http.StatusOK, out)
}

func linkResponse(l ShareLink) ShareLinkResponse {
	return ShareLinkResponse{
		ID:        l.ID,
		FileID:    l.FileID,
		Token:     l.Token,
		IsActive:  l.IsActive,
		CreatedAt: l.CreatedAt,
	}
}

func sharedFileResponse(f files.File) SharedFileResponse {
	return SharedFileResponse{
		ID:           f.ID,
		StorageID:    f.StorageID,
		OriginalName: f.OriginalName,
		SizeBytes:    f.SizeBytes,
		Extension:    f.Extension,
		MimeType:     f.MimeType,
		CreatedAt:    f.CreatedAt,
	}
}

func idParam(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, name), 10, 64)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		response.Error(w, http.StatusForbidden, "access denied")
	case errors.Is(err, ErrFileNotFound):
		response.Error(w, http.StatusNotFound, "file not found")
	case errors.Is(err, ErrLinkNotFound):
		response.Error(w, http.StatusNotFound, "share link not found or revoked")
	default:
		response.Error(w, http.StatusInternalServerError, "internal error")
	}
}
