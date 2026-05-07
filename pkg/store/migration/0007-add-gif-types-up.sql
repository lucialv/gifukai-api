ALTER TABLE gifs ADD COLUMN variant TEXT;
ALTER TABLE suggestions ADD COLUMN variant TEXT;

CREATE INDEX IF NOT EXISTS idx_gifs_action_variant_pairing_nsfw ON gifs (action, variant, pairing, nsfw);
