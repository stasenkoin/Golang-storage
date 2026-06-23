package shares

import "time"

type ShareLinkResponse struct {
	ID        int64     `json:"id"`
	FileID    int64     `json:"file_id"`
	Token     string    `json:"token"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type SharedFileResponse struct {
	ID           int64     `json:"id"`
	StorageID    int64     `json:"storage_id"`
	OriginalName string    `json:"original_name"`
	SizeBytes    int64     `json:"size_bytes"`
	Extension    string    `json:"extension"`
	MimeType     string    `json:"mime_type"`
	CreatedAt    time.Time `json:"created_at"`
}

type AvailableFileResponse struct {
	ID            int64     `json:"id"`
	OriginalName  string    `json:"original_name"`
	SizeBytes     int64     `json:"size_bytes"`
	Extension     string    `json:"extension"`
	MimeType      string    `json:"mime_type"`
	SharedByEmail string    `json:"shared_by_email"`
	AccessedAt    time.Time `json:"accessed_at"`
}
