CREATE TABLE IF NOT EXISTS %[1]s (
    key TEXT PRIMARY KEY,
    data JSON NULL,
    data_jsonb JSONB NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s ADD COLUMN IF NOT EXISTS data_jsonb JSONB NULL;
ALTER TABLE %[1]s ALTER COLUMN data DROP NOT NULL;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = '%[1]s_data_xor'
    ) THEN
        ALTER TABLE %[1]s
            ADD CONSTRAINT %[1]s_data_xor
            CHECK ((data IS NULL) <> (data_jsonb IS NULL));
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS %[1]s_key_prefix_idx ON %[1]s (key text_pattern_ops);
CREATE INDEX IF NOT EXISTS %[1]s_data_jsonb_gin ON %[1]s USING gin (data_jsonb jsonb_path_ops);
