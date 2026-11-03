-- A reservation is written before any external Add. A crash or ambiguous Add
-- leaves it in place: operators must verify downloader state before releasing it.
CREATE TABLE tv_episode_admission (
 episode_id INTEGER PRIMARY KEY REFERENCES episode(id) ON DELETE CASCADE,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
