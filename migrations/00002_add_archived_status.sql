-- +goose Up
-- SQLite doesn't support ALTER CHECK constraint, so recreate the table
CREATE TABLE categories_new (
  id            INTEGER PRIMARY KEY,
  name          TEXT NOT NULL,
  vote_type     TEXT NOT NULL CHECK (vote_type IN ('single', 'ranked', 'approval')),
  status        TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'open', 'closed', 'archived')),
  show_results  TEXT NOT NULL DEFAULT 'after_close' CHECK (show_results IN ('live', 'after_close')),
  max_rank      INTEGER,
  created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO categories_new SELECT * FROM categories;
DROP TABLE categories;
ALTER TABLE categories_new RENAME TO categories;

-- +goose Down
CREATE TABLE categories_new (
  id            INTEGER PRIMARY KEY,
  name          TEXT NOT NULL,
  vote_type     TEXT NOT NULL CHECK (vote_type IN ('single', 'ranked', 'approval')),
  status        TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'open', 'closed')),
  show_results  TEXT NOT NULL DEFAULT 'after_close' CHECK (show_results IN ('live', 'after_close')),
  max_rank      INTEGER,
  created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO categories_new SELECT * FROM categories WHERE status != 'archived';
DROP TABLE categories;
ALTER TABLE categories_new RENAME TO categories;
