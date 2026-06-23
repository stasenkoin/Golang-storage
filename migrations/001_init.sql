CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS storages (
    id                  BIGSERIAL PRIMARY KEY,
    owner_id            BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    type                TEXT NOT NULL CHECK (type IN ('personal', 'global')),
    max_file_size_bytes BIGINT NOT NULL CHECK (max_file_size_bytes > 0),
    allowed_extensions  TEXT[] NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS files (
    id            BIGSERIAL PRIMARY KEY,
    storage_id    BIGINT NOT NULL REFERENCES storages(id) ON DELETE CASCADE,
    uploaded_by   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    original_name TEXT NOT NULL,
    stored_name   TEXT NOT NULL,
    disk_path     TEXT NOT NULL,
    size_bytes    BIGINT NOT NULL CHECK (size_bytes >= 0),
    extension     TEXT NOT NULL,
    mime_type     TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS storage_permissions (
    id         BIGSERIAL PRIMARY KEY,
    storage_id BIGINT NOT NULL REFERENCES storages(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission TEXT NOT NULL CHECK (permission IN ('read', 'write')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (storage_id, user_id)
);

CREATE TABLE IF NOT EXISTS share_links (
    id         BIGSERIAL PRIMARY KEY,
    file_id    BIGINT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_active  BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS shared_file_access (
    id            BIGSERIAL PRIMARY KEY,
    file_id       BIGINT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shared_by     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    share_link_id BIGINT NOT NULL REFERENCES share_links(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (file_id, user_id)
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    token      TEXT PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_storages_owner_id ON storages(owner_id);
CREATE INDEX IF NOT EXISTS idx_files_storage_id ON files(storage_id);
CREATE INDEX IF NOT EXISTS idx_storage_permissions_user_id ON storage_permissions(user_id);
CREATE INDEX IF NOT EXISTS idx_share_links_token ON share_links(token);
CREATE INDEX IF NOT EXISTS idx_shared_file_access_user_id ON shared_file_access(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
