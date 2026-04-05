CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    email TEXT,
    display_name TEXT NOT NULL,
    avatar_url TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_users_provider ON users (provider, provider_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);