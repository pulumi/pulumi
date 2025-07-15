CREATE TABLE IF NOT EXISTS %s (
    key TEXT PRIMARY KEY,
    data JSON NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS %s_key_prefix_idx ON %s (key text_pattern_ops); 