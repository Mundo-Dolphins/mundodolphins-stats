package videos

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type CandidateStore interface {
	LoadCandidates(ctx context.Context, limit int) ([]Candidate, error)
	SaveSearchResults(ctx context.Context, game GameRef, query string, source string, candidates []Candidate) error
}

type SQLiteCandidateStore struct {
	db      *sql.DB
	writeMu sync.Mutex
	Now     func() time.Time
}

func NewSQLiteCandidateStore(path string) (*SQLiteCandidateStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("discovery sqlite path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create discovery sqlite dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open discovery sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &SQLiteCandidateStore{db: db, Now: time.Now}
	if err := store.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteCandidateStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteCandidateStore) init(ctx context.Context) error {
	stmts := []string{
		`PRAGMA busy_timeout = 10000`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = NORMAL`,
		`CREATE TABLE IF NOT EXISTS videos (
			video_id TEXT PRIMARY KEY,
			url TEXT NOT NULL,
			title TEXT,
			channel TEXT,
			duration_text TEXT,
			duration_seconds INTEGER,
			snippet TEXT,
			embeddable INTEGER,
			first_seen_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS searches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			game_id TEXT,
			query TEXT NOT NULL,
			source TEXT NOT NULL,
			searched_at TEXT NOT NULL,
			result_count INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS search_results (
			search_id INTEGER NOT NULL,
			video_id TEXT NOT NULL,
			position INTEGER NOT NULL,
			PRIMARY KEY (search_id, video_id),
			FOREIGN KEY (search_id) REFERENCES searches(id),
			FOREIGN KEY (video_id) REFERENCES videos(video_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_videos_last_seen ON videos(last_seen_at DESC)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("init discovery sqlite: %w", err)
		}
	}
	return nil
}

func (s *SQLiteCandidateStore) LoadCandidates(ctx context.Context, limit int) ([]Candidate, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("discovery sqlite store is nil")
	}
	if limit <= 0 {
		limit = 5000
	}
	rows, err := s.db.QueryContext(ctx, `SELECT video_id, url, title, channel, duration_text, duration_seconds, snippet, embeddable FROM videos ORDER BY last_seen_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("load discovery candidates: %w", err)
	}
	defer rows.Close()

	var out []Candidate
	for rows.Next() {
		var c Candidate
		var embeddable sql.NullInt64
		if err := rows.Scan(&c.VideoID, &c.URL, &c.Title, &c.Channel, &c.DurationText, &c.DurationSec, &c.Snippet, &embeddable); err != nil {
			return nil, fmt.Errorf("scan discovery candidate: %w", err)
		}
		if embeddable.Valid {
			value := embeddable.Int64 != 0
			c.Embeddable = &value
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate discovery candidates: %w", err)
	}
	return out, nil
}

func (s *SQLiteCandidateStore) LoadCandidatesForGame(ctx context.Context, game GameRef, limit int) ([]Candidate, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("discovery sqlite store is nil")
	}
	query := `SELECT v.video_id, v.url, v.title, v.channel, v.duration_text, v.duration_seconds, v.snippet, v.embeddable
		FROM searches s
		JOIN search_results sr ON sr.search_id = s.id
		JOIN videos v ON v.video_id = sr.video_id
		WHERE s.game_id = ?
		ORDER BY s.searched_at DESC, sr.position ASC`
	args := []any{game.GameID}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load discovery candidates for game %s: %w", game.GameID, err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	var out []Candidate
	for rows.Next() {
		var c Candidate
		var embeddable sql.NullInt64
		if err := rows.Scan(&c.VideoID, &c.URL, &c.Title, &c.Channel, &c.DurationText, &c.DurationSec, &c.Snippet, &embeddable); err != nil {
			return nil, fmt.Errorf("scan discovery candidate for game %s: %w", game.GameID, err)
		}
		key := normalizeCandidateID(c)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if embeddable.Valid {
			value := embeddable.Int64 != 0
			c.Embeddable = &value
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate discovery candidates for game %s: %w", game.GameID, err)
	}
	return out, nil
}

func (s *SQLiteCandidateStore) SaveSearchResults(ctx context.Context, game GameRef, query string, source string, candidates []Candidate) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("discovery sqlite store is nil")
	}
	if len(candidates) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if strings.TrimSpace(source) == "" {
		source = "unknown"
	}
	now := s.now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin discovery sqlite tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `INSERT INTO searches(game_id, query, source, searched_at, result_count) VALUES(?, ?, ?, ?, ?)`, game.GameID, query, source, now, len(candidates))
	if err != nil {
		return fmt.Errorf("insert discovery search: %w", err)
	}
	searchID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("get discovery search id: %w", err)
	}

	for i, candidate := range candidates {
		videoID := normalizeCandidateID(candidate)
		if videoID == "" {
			continue
		}
		candidate.VideoID = videoID
		if candidate.URL == "" {
			candidate.URL = "https://www.youtube.com/watch?v=" + videoID
		}
		embeddable := nullableBool(candidate.Embeddable)
		if _, err := tx.ExecContext(ctx, upsertDiscoveryVideoSQL,
			candidate.VideoID,
			candidate.URL,
			candidate.Title,
			candidate.Channel,
			candidate.DurationText,
			candidate.DurationSec,
			candidate.Snippet,
			embeddable,
			now,
			now,
		); err != nil {
			return fmt.Errorf("upsert discovery video %s: %w", videoID, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO search_results(search_id, video_id, position) VALUES(?, ?, ?)`, searchID, videoID, i+1); err != nil {
			return fmt.Errorf("insert discovery search result %s: %w", videoID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit discovery sqlite tx: %w", err)
	}
	return nil
}

const upsertDiscoveryVideoSQL = `INSERT INTO videos(video_id, url, title, channel, duration_text, duration_seconds, snippet, embeddable, first_seen_at, last_seen_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(video_id) DO UPDATE SET
	url = CASE WHEN excluded.url <> '' THEN excluded.url ELSE videos.url END,
	title = CASE WHEN excluded.title <> '' THEN excluded.title ELSE videos.title END,
	channel = CASE WHEN excluded.channel <> '' THEN excluded.channel ELSE videos.channel END,
	duration_text = CASE WHEN excluded.duration_text <> '' THEN excluded.duration_text ELSE videos.duration_text END,
	duration_seconds = CASE WHEN excluded.duration_seconds > 0 THEN excluded.duration_seconds ELSE videos.duration_seconds END,
	snippet = CASE WHEN excluded.snippet <> '' THEN excluded.snippet ELSE videos.snippet END,
	embeddable = COALESCE(excluded.embeddable, videos.embeddable),
	last_seen_at = excluded.last_seen_at`

func (s *SQLiteCandidateStore) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func normalizeCandidateID(candidate Candidate) string {
	if id := strings.TrimSpace(candidate.VideoID); id != "" {
		return id
	}
	return extractYouTubeVideoID(candidate.URL)
}

func nullableBool(value *bool) any {
	if value == nil {
		return nil
	}
	if *value {
		return 1
	}
	return 0
}
