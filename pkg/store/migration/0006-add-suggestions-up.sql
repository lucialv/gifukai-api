CREATE TABLE IF NOT EXISTS suggestions (
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

CREATE INDEX IF NOT EXISTS idx_suggestions_status ON suggestions (status);
CREATE INDEX IF NOT EXISTS idx_suggestions_user ON suggestions (user_id);