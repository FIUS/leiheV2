CREATE TABLE IF NOT EXISTS app_settings (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    require_deposit BOOLEAN NOT NULL DEFAULT FALSE
);

INSERT INTO app_settings (id, require_deposit)
VALUES (1, FALSE)
ON CONFLICT (id) DO NOTHING;
