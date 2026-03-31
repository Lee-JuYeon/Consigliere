package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

const DBDir = ".ledger"
const DBFile = "index.db"

type Store struct {
	db      *sql.DB
	baseDir string
}

func (s *Store) BaseDir() string {
	return s.baseDir
}

func New(baseDir string) (*Store, error) {
	dbDir := filepath.Join(baseDir, DBDir)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dbPath := filepath.Join(dbDir, DBFile)
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	s := &Store{db: db, baseDir: baseDir}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS chunks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file TEXT NOT NULL,
		heading TEXT,
		content TEXT NOT NULL,
		content_hash TEXT NOT NULL,
		file_type TEXT NOT NULL,
		date TEXT,
		tags TEXT DEFAULT '',
		status TEXT DEFAULT 'active',
		line_start INTEGER,
		line_end INTEGER,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS metadata (
		file TEXT PRIMARY KEY,
		file_hash TEXT NOT NULL,
		file_type TEXT NOT NULL,
		chunk_count INTEGER DEFAULT 0,
		last_modified TIMESTAMP,
		last_indexed TIMESTAMP
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
		content,
		heading,
		tags,
		content='chunks',
		content_rowid='id'
	);

	CREATE INDEX IF NOT EXISTS idx_chunks_file ON chunks(file);
	CREATE INDEX IF NOT EXISTS idx_chunks_type ON chunks(file_type);
	CREATE INDEX IF NOT EXISTS idx_chunks_hash ON chunks(content_hash);
	CREATE INDEX IF NOT EXISTS idx_chunks_date ON chunks(date);

	CREATE TABLE IF NOT EXISTS vectors (
		chunk_id INTEGER PRIMARY KEY,
		embedding BLOB NOT NULL,
		model TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (chunk_id) REFERENCES chunks(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		commit_hash TEXT NOT NULL,
		label TEXT DEFAULT '',
		total_files INTEGER,
		total_chunks INTEGER,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS snapshot_chunks (
		snapshot_id INTEGER NOT NULL,
		chunk_id INTEGER NOT NULL,
		file TEXT NOT NULL,
		heading TEXT,
		content_hash TEXT NOT NULL,
		file_type TEXT NOT NULL,
		PRIMARY KEY (snapshot_id, chunk_id),
		FOREIGN KEY (snapshot_id) REFERENCES snapshots(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_snapshots_commit ON snapshots(commit_hash);
	`
	_, err := s.db.Exec(schema)
	return err
}
