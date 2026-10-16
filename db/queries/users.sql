-- name: CreateUser :one
-- id and created_at come from column defaults.
INSERT INTO users (email, display_name, password_hash)
VALUES (@email, @display_name, @password_hash)
RETURNING id, email, display_name, created_at;

-- name: GetUserByEmail :one
-- Used by login. This is the only query that returns password_hash.
SELECT id, email, display_name, password_hash, created_at
FROM users
WHERE email = @email;

-- name: GetUserByID :one
SELECT id, email, display_name, created_at
FROM users
WHERE id = @id;
