-- name: ListItems :many
SELECT * FROM items ORDER BY name;

-- name: GetItem :one
SELECT * FROM items WHERE id = $1 LIMIT 1;

-- name: CreateItem :one
INSERT INTO items (
    name, isbn, description, image_url, custom_image_url,
    min_players, max_players, min_playtime, max_playtime, status
) VALUES (
             $1, $2, $3, $4, $5, $6, $7, $8, $9
         ) RETURNING *;

-- name: UpdateItemStatus :exec
UPDATE items SET status = $2 WHERE id = $1;

-- name: CreateBooking :one
INSERT INTO bookings (
    item_id, user_id, guest_name, guest_email, start_date, end_date, status
) VALUES (
             $1, $2, $3, $4, $5, $6, $7
         ) RETURNING *;