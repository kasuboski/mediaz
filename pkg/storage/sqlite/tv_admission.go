package sqlite

import (
	"context"
	"fmt"
)

// ReserveTVEpisodes atomically claims an entire release coverage set. These rows
// intentionally survive restarts, ambiguous submission, and transition failures.
func (s *SQLite) ReserveTVEpisodes(ctx context.Context, ids []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		// A season job (or a surviving pack linkage on any sibling) has
		// conservative whole-season coverage, even if child flags changed.
		var covered int
		err = tx.QueryRowContext(ctx, `SELECT
 EXISTS(SELECT 1 FROM season_transition st JOIN episode target ON target.season_id=st.season_id
 WHERE target.id=? AND st.most_recent=1 AND st.to_state='downloading' AND (st.download_id IS NOT NULL OR st.download_client_id IS NOT NULL))
 OR EXISTS(SELECT 1 FROM episode sibling JOIN episode target ON sibling.season_id=target.season_id
 JOIN episode_transition et ON et.episode_id=sibling.id
 WHERE target.id=? AND et.most_recent=1 AND et.to_state='downloading' AND et.is_entire_season_download=1)`, id, id).Scan(&covered)
		if err != nil {
			return err
		}
		if covered != 0 {
			return fmt.Errorf("episode %d is covered by an active season job", id)
		}
		var state string
		err = tx.QueryRowContext(ctx, `SELECT to_state FROM episode_transition WHERE episode_id=? AND most_recent=1`, id).Scan(&state)
		if err != nil {
			return err
		}
		if state != "missing" {
			return fmt.Errorf("episode %d is no longer missing", id)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO tv_episode_admission(episode_id) VALUES (?)`, id); err != nil {
			return fmt.Errorf("episode %d admission blocked; verify downloader activity before recovery: %w", id, err)
		}
	}
	return tx.Commit()
}

// ReleaseTVEpisodes is safe only before Add, or after all submission linkage has
// been durably recorded. Never call it merely because Add returned an error.
func (s *SQLite) ReleaseTVEpisodes(ctx context.Context, ids []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err = tx.ExecContext(ctx, `DELETE FROM tv_episode_admission WHERE episode_id=?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
