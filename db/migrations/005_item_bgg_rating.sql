ALTER TABLE items
    ADD COLUMN IF NOT EXISTS bgg_rating NUMERIC(3,1)
    CHECK (bgg_rating IS NULL OR bgg_rating BETWEEN 1 AND 10);
