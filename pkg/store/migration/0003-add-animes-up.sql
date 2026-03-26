CREATE TABLE IF NOT EXISTS animes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS gifs_new (
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

INSERT INTO gifs_new (id, action, pairing, r2_key, content_type, size_bytes, tags, nsfw, created_at)
    SELECT id, action, pairing, r2_key, content_type, size_bytes, tags, nsfw, created_at FROM gifs;

DROP TABLE gifs;

ALTER TABLE gifs_new RENAME TO gifs;

CREATE INDEX IF NOT EXISTS idx_gifs_action_pairing ON gifs (action, pairing);
CREATE INDEX IF NOT EXISTS idx_gifs_action_pairing_nsfw ON gifs (action, pairing, nsfw);
CREATE INDEX IF NOT EXISTS idx_gifs_action ON gifs (action);
CREATE INDEX IF NOT EXISTS idx_gifs_anime_id ON gifs (anime_id);