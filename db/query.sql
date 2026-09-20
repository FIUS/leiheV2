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
INSERT INTO users (oidc_sub, email, name, role, last_login)
VALUES ($1, $2, $3, $4, NOW())
    ON CONFLICT (oidc_sub)
DO UPDATE SET
    email = EXCLUDED.email,
           name = EXCLUDED.name,
           role = EXCLUDED.role,
           last_login = NOW()
           RETURNING *;