CREATE TABLE IF NOT EXISTS gifs_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    action TEXT NOT NULL,
    pairing TEXT NOT NULL,
    r2_key TEXT NOT NULL UNIQUE,
    content_type TEXT NOT NULL DEFAULT 'image/gif',
    size_bytes INTEGER NOT NULL DEFAULT 0,
    tags TEXT,
    nsfw BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO gifs_old SELECT * FROM gifs;

DROP TABLE gifs;

ALTER TABLE gifs_old RENAME TO gifs;

CREATE INDEX IF NOT EXISTS idx_gifs_action_pairing ON gifs (action, pairing);
CREATE INDEX IF NOT EXISTS idx_gifs_action_pairing_nsfw ON gifs (action, pairing, nsfw);
CREATE INDEX IF NOT EXISTS idx_gifs_action ON gifs (action);