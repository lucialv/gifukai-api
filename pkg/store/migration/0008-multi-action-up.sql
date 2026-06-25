CREATE TABLE gif_actions (
    gif_id INTEGER NOT NULL REFERENCES gifs(id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    PRIMARY KEY (gif_id, action)
);

CREATE INDEX idx_gif_actions_action ON gif_actions (action);

-- backfill existing single action into the join table
INSERT INTO gif_actions (gif_id, action) SELECT id, action FROM gifs;
