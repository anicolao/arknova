package projections

import (
	"database/sql"
	"encoding/json"

	"github.com/anicolao/arknova/internal/game"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS projection_state (
game_id TEXT PRIMARY KEY, next_offset INTEGER NOT NULL, revision INTEGER NOT NULL, state_json TEXT NOT NULL
)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Put(state game.State, offset int64) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO projection_state(game_id,next_offset,revision,state_json) VALUES(?,?,?,?)
ON CONFLICT(game_id) DO UPDATE SET next_offset=excluded.next_offset,revision=excluded.revision,state_json=excluded.state_json`, state.GameID, offset, state.Revision, string(data))
	return err
}

func (s *Store) DeleteAll() error { _, err := s.db.Exec(`DELETE FROM projection_state`); return err }
func (s *Store) Close() error     { return s.db.Close() }
