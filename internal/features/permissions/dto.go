package permissions

import "time"

type GrantRequest struct {
	Email      string `json:"email"`
	Permission string `json:"permission"`
}
type UpdatePermissionRequest struct {
	Permission string `json:"permission"`
}

type PermissionResponse struct {
	ID         int64     `json:"id"`
	StorageID  int64     `json:"storage_id"`
	UserID     int64     `json:"user_id"`
	Permission string    `json:"permission"`
	CreatedAt  time.Time `json:"created_at"`
}

type GrantedAccessResponse struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	UserEmail  string    `json:"user_email"`
	Permission string    `json:"permission"`
	CreatedAt  time.Time `json:"created_at"`
}
