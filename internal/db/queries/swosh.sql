-- name: GetSwosh :one
SELECT * FROM swosh
WHERE id = $1;

-- name: ListSwoshes :many
-- Latest expiry first and links that never expire last, like the old Mongo sort.
SELECT * FROM swosh
ORDER BY expires_on DESC NULLS LAST, created_at DESC;

-- name: CreateSwosh :one
INSERT INTO swosh (id, payee, amount, description, expires_on, payee_editable, amount_editable, message_editable)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateSwosh :execrows
-- Renames the link when new_id differs from id.
UPDATE swosh
SET id = @new_id, payee = @payee, amount = @amount, description = @description
WHERE id = @id;

-- name: DeleteSwosh :execrows
DELETE FROM swosh
WHERE id = $1;

-- name: DeleteExpiredSwoshes :many
DELETE FROM swosh
WHERE expires_on < sqlc.arg(now)::timestamptz
RETURNING id;

-- name: ImportSwosh :execrows
-- Leaves an existing link with the same id untouched, so imports can be re-run.
INSERT INTO swosh (id, payee, amount, description, expires_on, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO NOTHING;
