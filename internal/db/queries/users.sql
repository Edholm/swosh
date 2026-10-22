-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1;

-- name: CreateUser :execrows
-- Does nothing if the username is taken; a row count of 0 means it existed.
INSERT INTO users (username, password, enabled, authorities)
VALUES ($1, $2, $3, $4)
ON CONFLICT (username) DO NOTHING;
