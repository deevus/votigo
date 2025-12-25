-- Queries for sqlc code generation

-- name: GetCategory :one
SELECT * FROM categories WHERE id = ?;
