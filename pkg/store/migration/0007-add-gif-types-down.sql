DROP INDEX IF EXISTS idx_gifs_action_variant_pairing_nsfw;

CREATE TABLE IF NOT EXISTS gifs_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    action TEXT NOT NULL,
    pairing TEXT NOT NULL CHECK (pairing IN ('f', 'm', 'ff', 'mm', 'fm', 'mf')),
    r2_key TEXT NOT NULL UNIQUE,
    content_type TEXT NOT NULL DEFAULT 'image/gif',
    size_bytes INTEGER NOT NULL DEFAULT 0,
    tags TEXT,
    nsfw BOOLEAN NOT NULL DEFAULT 0,
    anime_id INTEGER REFERENCES animes(id) ON DELETE SET NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO gifs_old (id, action, pairing, r2_key, content_type, size_bytes, tags, nsfw, anime_id, created_at)
    SELECT id, action, pairing, r2_key, content_type, size_bytes, tags, nsfw, anime_id, created_at FROM gifs;

DROP TABLE gifs;

ALTER TABLE gifs_old RENAME TO gifs;

CREATE INDEX IF NOT EXISTS idx_gifs_action_pairing ON gifs (action, pairing);
CREATE INDEX IF NOT EXISTS idx_gifs_action_pairing_nsfw ON gifs (action, pairing, nsfw);
CREATE INDEX IF NOT EXISTS idx_gifs_action ON gifs (action);
CREATE INDEX IF NOT EXISTS idx_gifs_anime_id ON gifs (anime_id);

CREATE TABLE IF NOT EXISTS suggestions_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_key TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL DEFAULT 0,
    action TEXT NOT NULL,
    pairing TEXT NOT NULL CHECK (pairing IN ('f', 'm', 'ff', 'mm', 'fm', 'mf')),
    anime TEXT,
    tags TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO suggestions_old (id, user_id, file_key, content_type, size_bytes, action, pairing, anime, tags, status, created_at)
    SELECT id, user_id, file_key, content_type, size_bytes, action, pairing, anime, tags, status, created_at FROM suggestions;

DROP TABLE suggestions;

ALTER TABLE suggestions_old RENAME TO suggestions;

CREATE INDEX IF NOT EXISTS idx_suggestions_status ON suggestions (status);
CREATE INDEX IF NOT EXISTS idx_suggestions_user ON suggestions (user_id);
