-- NOTE: This file must be kept in sync with migrations/
-- It is used by sqlc for code generation only.

CREATE TABLE categories (
  id            INTEGER PRIMARY KEY,
  name          TEXT NOT NULL,
  vote_type     TEXT NOT NULL CHECK (vote_type IN ('single', 'ranked', 'approval')),
  status        TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'open', 'closed', 'archived')),
  show_results  TEXT NOT NULL DEFAULT 'after_close' CHECK (show_results IN ('live', 'after_close')),
  max_rank      INTEGER,
  created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE options (
  id          INTEGER PRIMARY KEY,
  category_id INTEGER NOT NULL,
  name        TEXT NOT NULL,
  sort_order  INTEGER DEFAULT 0,
  FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
);

CREATE TABLE votes (
  id          INTEGER PRIMARY KEY,
  category_id INTEGER NOT NULL,
  nickname    TEXT NOT NULL,
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(category_id, nickname),
  FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
);

CREATE TABLE vote_selections (
  id        INTEGER PRIMARY KEY,
  vote_id   INTEGER NOT NULL,
  option_id INTEGER NOT NULL,
  rank      INTEGER,
  FOREIGN KEY (vote_id) REFERENCES votes(id) ON DELETE CASCADE,
  FOREIGN KEY (option_id) REFERENCES options(id) ON DELETE CASCADE
);

-- Indexes for query performance
CREATE INDEX idx_options_category ON options(category_id);
CREATE INDEX idx_votes_category ON votes(category_id);
CREATE INDEX idx_vote_selections_vote ON vote_selections(vote_id);
CREATE INDEX idx_vote_selections_option ON vote_selections(option_id);
