-- ENUMS
CREATE TYPE user_role AS ENUM ('user', 'maintainer', 'admin');
CREATE TYPE item_status AS ENUM ('available', 'rented', 'maintenance', 'lost');
CREATE TYPE booking_status AS ENUM ('pending', 'confirmed', 'active', 'returned', 'cancelled');

-- USERS
CREATE TABLE users (
                       id SERIAL PRIMARY KEY,
                       email VARCHAR(255) UNIQUE NOT NULL,
                       first_name VARCHAR(255) NOT NULL,
                       last_name VARCHAR(255) NOT NULL,
                       oidc_sub VARCHAR(255) UNIQUE NOT NULL,
                       password_hash TEXT,
                       email_verified BOOLEAN NOT NULL DEFAULT FALSE,
                       role user_role DEFAULT 'user' NOT NULL,
                       created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                       last_login TIMESTAMP
);

CREATE TABLE email_verification_tokens (
    token_hash CHAR(64) PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_email_verification_tokens_expiry ON email_verification_tokens(expires_at);

-- CATEGORIES
CREATE TABLE categories (
                            id SERIAL PRIMARY KEY,
                            name VARCHAR(100) UNIQUE NOT NULL
);

-- ITEMS (Spiele)
CREATE TABLE items (
                       id SERIAL PRIMARY KEY,
                       name VARCHAR(255) NOT NULL,
                       isbn VARCHAR(100) UNIQUE,
                       description TEXT,
                       image_url VARCHAR(255),
                       custom_image_url VARCHAR(255),
                       room VARCHAR(100),
                       shelf VARCHAR(100),
                       min_players INT NOT NULL DEFAULT 0,
                       max_players INT NOT NULL DEFAULT 0,
                       min_playtime INT NOT NULL DEFAULT 0,
                       max_playtime INT NOT NULL DEFAULT 0,
                       bgg_rating NUMERIC(3,1) CHECK (bgg_rating IS NULL OR bgg_rating BETWEEN 1 AND 10),
                       status item_status NOT NULL DEFAULT 'available',
                       is_expansion BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE storage_shelves (
    code VARCHAR(100) PRIMARY KEY,
    sort_order INTEGER NOT NULL
);

CREATE UNIQUE INDEX idx_storage_shelves_code_ci ON storage_shelves (LOWER(code));

-- ITEM_CATEGORIES
CREATE TABLE item_categories (
                                 item_id INT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
                                 category_id INT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
                                 PRIMARY KEY (item_id, category_id)
);

-- BOOKINGS (Der Buchungs-Deckel)
CREATE TABLE bookings (
                          id SERIAL PRIMARY KEY,
                          public_token VARCHAR(12) UNIQUE NOT NULL,
                          user_id INT REFERENCES users(id) ON DELETE SET NULL,
                          created_by_user_id INT REFERENCES users(id) ON DELETE SET NULL,
                          guest_name VARCHAR(255),
                          guest_email VARCHAR(255),
                          deposit TEXT,
                          user_deleted BOOLEAN NOT NULL DEFAULT FALSE,
                          start_date TIMESTAMPTZ NOT NULL,
                          end_date TIMESTAMPTZ NOT NULL,
                          status booking_status NOT NULL DEFAULT 'pending',
                          created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE app_settings (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    require_deposit BOOLEAN NOT NULL DEFAULT FALSE,
    module_barcodes_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    email_booking_received BOOLEAN NOT NULL DEFAULT TRUE,
    email_booking_review BOOLEAN NOT NULL DEFAULT TRUE,
    email_booking_updates BOOLEAN NOT NULL DEFAULT TRUE,
    ignored_email_addresses TEXT NOT NULL DEFAULT '',
    booking_history_days INT NOT NULL DEFAULT 0 CHECK (booking_history_days >= 0),
    bgg_sync_limit INT NOT NULL DEFAULT 20 CHECK (bgg_sync_limit BETWEEN 1 AND 100),
    bgg_load_images BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO app_settings (
    id, require_deposit, module_barcodes_enabled,
    email_booking_received, email_booking_review, email_booking_updates, ignored_email_addresses
) VALUES (1, FALSE, FALSE, TRUE, TRUE, TRUE, '');

-- BOOKING_ITEMS (Der Korb-Inhalt)
CREATE TABLE booking_items (
                               booking_id INT NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
                               item_id INT NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
                               PRIMARY KEY (booking_id, item_id)
);

-- INDIZES
CREATE INDEX idx_items_status ON items(status);
CREATE INDEX idx_items_players ON items(min_players, max_players);
CREATE INDEX idx_items_playtime ON items(min_playtime, max_playtime);
CREATE INDEX idx_bookings_status ON bookings(status);
CREATE INDEX idx_bookings_token ON bookings(public_token);
CREATE INDEX idx_bookings_pending_start_date ON bookings(start_date) WHERE status = 'pending';

CREATE FUNCTION mark_bookings_user_deleted() RETURNS trigger AS $$
BEGIN
    UPDATE bookings SET user_deleted = TRUE WHERE user_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_mark_bookings_deleted
    BEFORE DELETE ON users
    FOR EACH ROW EXECUTE FUNCTION mark_bookings_user_deleted();
