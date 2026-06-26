CREATE TABLE action_aliases (
    alias   TEXT PRIMARY KEY,
    action  TEXT NOT NULL,
    variant TEXT
);

CREATE INDEX idx_action_aliases_action ON action_aliases (action);
