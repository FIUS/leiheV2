-- name: ListItems :many
SELECT * FROM items ORDER BY name;

-- name: GetItem :one
SELECT * FROM items WHERE id = $1 LIMIT 1;

-- name: CreateItem :one
INSERT INTO items (
    name, isbn, description, image_url, custom_image_url,
    min_players, max_players, min_playtime, max_playtime, status, room, shelf
) VALUES (
             $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
         ) RETURNING *;

-- name: UpdateItemStatus :exec
UPDATE items SET status = $2 WHERE id = $1;

-- name: DeleteItem :exec
DELETE FROM items WHERE id = $1;

-- name: UpdateItemCustomImage :exec
UPDATE items SET custom_image_url = $1 WHERE id = $2;

-- name: UpdateItem :exec
UPDATE items
SET name = $2, min_players = $3, max_players = $4, min_playtime = $5, max_playtime = $6, room = $7, shelf = $8
WHERE id = $1;

-- name: ListCategories :many
SELECT * FROM categories ORDER BY name;

-- name: GetOrCreateCategory :one
INSERT INTO categories (name)
VALUES ($1)
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING *;

-- name: AddItemCategory :exec
INSERT INTO item_categories (item_id, category_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ClearItemCategories :exec
DELETE FROM item_categories WHERE item_id = $1;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = $1;

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
           role = CASE
                      WHEN users.role IN ('admin', 'maintainer') AND EXCLUDED.role = 'user'
                          THEN users.role
                      ELSE EXCLUDED.role
                  END,
           last_login = NOW()
           RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 LIMIT 1;

-- name: CreateLocalUser :one
INSERT INTO users (
    email, first_name, last_name, oidc_sub, password_hash, email_verified, role
) VALUES (
    $1, $2, $3, $4, $5, $6, 'user'
) RETURNING *;

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
SELECT b.start_date, b.end_date, b.status
FROM bookings b
         JOIN booking_items bi ON b.id = bi.booking_id
WHERE bi.item_id = $1 AND b.end_date >= CURRENT_DATE
ORDER BY b.start_date ASC;

-- name: GetItemHistory :many
SELECT
    b.id, b.start_date, b.end_date, b.status,
    b.guest_name, u.first_name, u.last_name
FROM bookings b
         JOIN booking_items bi ON b.id = bi.booking_id
         LEFT JOIN users u ON b.user_id = u.id
WHERE bi.item_id = $1
ORDER BY b.start_date DESC;

-- name: CreateBooking :one
INSERT INTO bookings (
    public_token, user_id, guest_name, guest_email, start_date, end_date, status
) VALUES (
             $1, $2, $3, $4, $5, $6, $7
         ) RETURNING *;

-- name: AddBookingItem :exec
INSERT INTO booking_items (
    booking_id, item_id
) VALUES (
             $1, $2
         );

-- name: GetBookingByToken :one
SELECT * FROM bookings
WHERE public_token = $1 LIMIT 1;

-- name: GetItemsForBooking :many
SELECT i.* FROM items i
                    JOIN booking_items bi ON i.id = bi.item_id
WHERE bi.booking_id = $1;

-- name: UpdateBookingStatus :exec
UPDATE bookings
SET status = $2
WHERE id = $1;

-- name: CheckItemCollisions :many
SELECT bi.item_id, i.name
FROM booking_items bi
         JOIN bookings b ON b.id = bi.booking_id
         JOIN items i ON i.id = bi.item_id
WHERE bi.item_id = ANY($1::int[])
  AND b.status IN ('pending', 'active')
  AND b.start_date <= $2
  AND b.end_date >= $3;

-- name: LockItemsForBooking :exec
SELECT pg_advisory_xact_lock(item_id::bigint)
FROM unnest($1::int[]) AS item_id;