package storages

type CreateStorageRequest struct {
	Name              string   `json:"name"`
	Type              string   `json:"type"`
	MaxFileSizeMB     int64    `json:"max_file_size_mb"`
	AllowedExtensions []string `json:"allowed_extensions"`
}
type UpdateStorageRequest struct {
	Name              string   `json:"name"`
	MaxFileSizeMB     int64    `json:"max_file_size_mb"`
	AllowedExtensions []string `json:"allowed_extensions"`
}
