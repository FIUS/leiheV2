-- Mapping from OIDC role types to sxstem roles
CREATE TYPE user_role AS ENUM ('user', 'maintainer', 'admin');

CREATE TABLE users (
                       id SERIAL PRIMARY KEY,
                       email VARCHAR(255) UNIQUE NOT NULL,
                       first_name VARCHAR(255) NOT NULL,
                       last_name VARCHAR(255) NOT NULL,
                       oidc_sub VARCHAR(255) UNIQUE NOT NULL,
                       role user_role DEFAULT 'user' NOT NULL,
                       created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                       last_login TIMESTAMP
);

CREATE TABLE categories (
                            id SERIAL PRIMARY KEY,
                            name VARCHAR(100) UNIQUE NOT NULL
);

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

                       status VARCHAR(50) NOT NULL DEFAULT 'available'
                           CHECK (status IN ('available', 'rented', 'maintenance', 'lost'))
);

CREATE TABLE item_categories (
                                 item_id INT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
                                 category_id INT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
                                 PRIMARY KEY (item_id, category_id)
);

CREATE TABLE bookings (
                          id SERIAL PRIMARY KEY,
                          item_id INT NOT NULL REFERENCES items(id) ON DELETE RESTRICT,

                          user_id INT REFERENCES users(id) ON DELETE SET NULL,
                          guest_name VARCHAR(255),
                          guest_email VARCHAR(255),

                          start_date TIMESTAMPTZ NOT NULL,
                          end_date TIMESTAMPTZ NOT NULL,

                          status VARCHAR(50) NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending', 'active', 'returned', 'cancelled')),

                          created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_items_status ON items(status);
CREATE INDEX idx_items_players ON items(min_players, max_players);
CREATE INDEX idx_items_playtime ON items(min_playtime, max_playtime);
CREATE INDEX idx_bookings_status ON bookings(status);
