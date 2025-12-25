-- Queries for sqlc code generation

-- Category queries

-- name: CreateCategory :one
INSERT INTO categories (name, vote_type, status, show_results, max_rank)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetCategory :one
SELECT * FROM categories WHERE id = ?;

-- name: ListCategories :many
SELECT * FROM categories ORDER BY created_at DESC;

-- name: ListOpenCategories :many
SELECT * FROM categories WHERE status = 'open' ORDER BY created_at DESC;

-- name: ListCategoriesExcludeArchived :many
SELECT * FROM categories WHERE status != 'archived' ORDER BY id;

-- name: ListCategoriesWithResults :many
SELECT * FROM categories
WHERE (show_results = 'live' AND status = 'open')
   OR (show_results = 'after_close' AND status = 'closed')
ORDER BY id;

-- name: ArchiveCategory :exec
UPDATE categories SET status = 'archived' WHERE id = ?;

-- name: UpdateCategoryStatus :exec
UPDATE categories SET status = ? WHERE id = ?;

-- name: UpdateCategory :exec
UPDATE categories SET name = ?, vote_type = ?, show_results = ?, max_rank = ? WHERE id = ?;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = ?;

-- Option queries

-- name: CreateOption :one
INSERT INTO options (category_id, name, sort_order)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetOption :one
SELECT * FROM options WHERE id = ?;

-- name: ListOptionsByCategory :many
SELECT * FROM options WHERE category_id = ? ORDER BY sort_order, id;

-- name: DeleteOption :exec
DELETE FROM options WHERE id = ?;

-- name: CountOptionsByCategory :one
SELECT COUNT(*) FROM options WHERE category_id = ?;

-- Vote queries

-- name: UpsertVote :one
INSERT INTO votes (category_id, nickname)
VALUES (?, ?)
ON CONFLICT(category_id, nickname) DO UPDATE SET created_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetVoteByNickname :one
SELECT * FROM votes WHERE category_id = ? AND nickname = ?;

-- name: DeleteVoteSelections :exec
DELETE FROM vote_selections WHERE vote_id = ?;

-- name: CreateVoteSelection :exec
INSERT INTO vote_selections (vote_id, option_id, rank)
VALUES (?, ?, ?);

-- name: CountVotesByCategory :one
SELECT COUNT(*) FROM votes WHERE category_id = ?;

-- name: ListVotersByCategory :many
SELECT nickname FROM votes WHERE category_id = ? ORDER BY created_at;

-- Tally queries

-- name: TallySimple :many
SELECT o.id, o.name, COUNT(vs.id) as votes
FROM options o
LEFT JOIN vote_selections vs ON vs.option_id = o.id
WHERE o.category_id = sqlc.arg(category_id)
GROUP BY o.id
ORDER BY votes DESC, o.sort_order, o.id;

-- name: TallyRanked :many
SELECT o.id, o.name,
       COALESCE(SUM(sqlc.arg(max_rank) - vs.rank + 1), 0) as points,
       COUNT(CASE WHEN vs.rank = 1 THEN 1 END) as first_place_votes
FROM options o
LEFT JOIN vote_selections vs ON vs.option_id = o.id
WHERE o.category_id = sqlc.arg(category_id)
GROUP BY o.id
ORDER BY points DESC, first_place_votes DESC, o.sort_order, o.id;
