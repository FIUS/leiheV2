-- name: ListItems :many
SELECT * FROM items ORDER BY name;

-- name: GetItem :one
SELECT * FROM items WHERE id = $1 LIMIT 1;

-- name: CreateItem :one
INSERT INTO items (
    name, isbn, description, image_url, custom_image_url,
    min_players, max_players, min_playtime, max_playtime, status
) VALUES (
             $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
         ) RETURNING *;

-- name: UpdateItemStatus :exec
UPDATE items SET status = $2 WHERE id = $1;

-- name: DeleteItem :exec
DELETE FROM items WHERE id = $1;

-- name: CreateBooking :one
INSERT INTO bookings (
    item_id, user_id, guest_name, guest_email, start_date, end_date, status
) VALUES (
             $1, $2, $3, $4, $5, $6, $7
         ) RETURNING *;

-- name: UpdateItemCustomImage :exec
UPDATE items SET custom_image_url = $1 WHERE id = $2;

-- name: UpdateItem :exec
UPDATE items
SET name = $2, min_players = $3, max_players = $4, min_playtime = $5, max_playtime = $6
WHERE id = $1;

-- name: ListCategories :many
SELECT * FROM categories ORDER BY name;

-- name: ListItemsWithCategories :many
SELECT items.*, COALESCE(string_agg(categories.name, ','), '')::text AS category_names
FROM items
         LEFT JOIN item_categories ON items.id = item_categories.item_id
         LEFT JOIN categories ON item_categories.category_id = categories.id
GROUP BY items.id
ORDER BY items.name;

-- name: UpsertUser :one
INSERT INTO users (oidc_sub, email, first_name, last_name, role, last_login)
VALUES ($1, $2, $3, $4, $5, NOW())
    ON CONFLICT (oidc_sub)
DO UPDATE SET
    email = EXCLUDED.email,
           first_name = EXCLUDED.first_name,
           last_name = EXCLUDED.last_name,
           -- WICHTIG: role wird hier nicht überschrieben, damit Admins ihre Rechte behalten!
           last_login = NOW()
           RETURNING *;

-- name: ListUsers :many
SELECT * FROM users ORDER BY last_name ASC, first_name ASC;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (
    first_name, last_name, email, oidc_sub, role
) VALUES (
             $1, $2, $3, $4, $5
         ) RETURNING *;

-- name: UpdateUser :exec
UPDATE users
SET first_name = $2, last_name = $3, email = $4, role = $5, oidc_sub = $6
WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: GetUpcomingBookingsForItem :many
SELECT start_date, end_date, status
FROM bookings
WHERE item_id = $1 AND end_date >= CURRENT_DATE
ORDER BY start_date ASC;