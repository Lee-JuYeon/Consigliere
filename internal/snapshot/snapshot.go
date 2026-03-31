package snapshot

import (
	"database/sql"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Snapshot struct {
	ID          int
	CommitHash  string
	Label       string
	TotalFiles  int
	TotalChunks int
	CreatedAt   time.Time
}

type ChunkDiff struct {
	File        string
	Heading     string
	ContentHash string
	FileType    string
	Change      string // "added", "removed", "modified"
}

type DiffResult struct {
	FromCommit string
	ToCommit   string
	Added      []ChunkDiff
	Removed    []ChunkDiff
	Summary    DiffSummary
}

type DiffSummary struct {
	FilesAdded   int
	FilesRemoved int
	ChunksAdded  int
	ChunksRemoved int
}

// GetCurrentCommit returns the current HEAD commit hash.
func GetCurrentCommit(baseDir string) (string, error) {
	out, err := exec.Command("git", "-C", baseDir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository or no commits: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ResolveRef resolves a git ref (HEAD, HEAD~5, tag, branch) to a commit hash.
func ResolveRef(baseDir, ref string) (string, error) {
	out, err := exec.Command("git", "-C", baseDir, "rev-parse", ref).Output()
	if err != nil {
		return "", fmt.Errorf("cannot resolve ref %q: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Create takes a snapshot of the current index state.
func Create(db *sql.DB, baseDir, label string) (*Snapshot, error) {
	commitHash, err := GetCurrentCommit(baseDir)
	if err != nil {
		return nil, err
	}

	// 현재 인덱스 통계
	var totalFiles, totalChunks int
	db.QueryRow("SELECT COUNT(*) FROM metadata").Scan(&totalFiles)
	db.QueryRow("SELECT COUNT(*) FROM chunks WHERE status = 'active'").Scan(&totalChunks)

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// 스냅샷 레코드 생성
	res, err := tx.Exec(
		"INSERT INTO snapshots (commit_hash, label, total_files, total_chunks) VALUES (?, ?, ?, ?)",
		commitHash, label, totalFiles, totalChunks,
	)
	if err != nil {
		return nil, fmt.Errorf("insert snapshot: %w", err)
	}
	snapID, _ := res.LastInsertId()

	// 현재 활성 청크를 스냅샷에 복사
	_, err = tx.Exec(`
		INSERT INTO snapshot_chunks (snapshot_id, chunk_id, file, heading, content_hash, file_type)
		SELECT ?, id, file, heading, content_hash, file_type
		FROM chunks WHERE status = 'active'
	`, snapID)
	if err != nil {
		return nil, fmt.Errorf("copy chunks to snapshot: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &Snapshot{
		ID:          int(snapID),
		CommitHash:  commitHash,
		Label:       label,
		TotalFiles:  totalFiles,
		TotalChunks: totalChunks,
		CreatedAt:   time.Now(),
	}, nil
}

// List returns all snapshots ordered by creation time.
func List(db *sql.DB) ([]Snapshot, error) {
	rows, err := db.Query(
		"SELECT id, commit_hash, label, total_files, total_chunks, created_at FROM snapshots ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snaps []Snapshot
	for rows.Next() {
		var s Snapshot
		if err := rows.Scan(&s.ID, &s.CommitHash, &s.Label, &s.TotalFiles, &s.TotalChunks, &s.CreatedAt); err != nil {
			return nil, err
		}
		snaps = append(snaps, s)
	}
	return snaps, nil
}

// FindByCommit finds a snapshot by commit hash (prefix match).
func FindByCommit(db *sql.DB, commitPrefix string) (*Snapshot, error) {
	var s Snapshot
	err := db.QueryRow(
		"SELECT id, commit_hash, label, total_files, total_chunks, created_at FROM snapshots WHERE commit_hash LIKE ? ORDER BY created_at DESC LIMIT 1",
		commitPrefix+"%",
	).Scan(&s.ID, &s.CommitHash, &s.Label, &s.TotalFiles, &s.TotalChunks, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("snapshot not found for %s", commitPrefix)
	}
	return &s, nil
}

// Diff compares two snapshots and returns what changed.
func Diff(db *sql.DB, fromID, toID int) (*DiffResult, error) {
	var fromCommit, toCommit string
	db.QueryRow("SELECT commit_hash FROM snapshots WHERE id = ?", fromID).Scan(&fromCommit)
	db.QueryRow("SELECT commit_hash FROM snapshots WHERE id = ?", toID).Scan(&toCommit)

	// toに있ってfromにないもの = added
	added, err := queryDiffChunks(db,
		`SELECT sc.file, sc.heading, sc.content_hash, sc.file_type
		 FROM snapshot_chunks sc
		 WHERE sc.snapshot_id = ?
		   AND sc.content_hash NOT IN (
		     SELECT content_hash FROM snapshot_chunks WHERE snapshot_id = ?
		   )`, toID, fromID)
	if err != nil {
		return nil, err
	}

	// fromに있ってtoにないもの = removed
	removed, err := queryDiffChunks(db,
		`SELECT sc.file, sc.heading, sc.content_hash, sc.file_type
		 FROM snapshot_chunks sc
		 WHERE sc.snapshot_id = ?
		   AND sc.content_hash NOT IN (
		     SELECT content_hash FROM snapshot_chunks WHERE snapshot_id = ?
		   )`, fromID, toID)
	if err != nil {
		return nil, err
	}

	// 파일 단위 통계
	addedFiles := uniqueFiles(added)
	removedFiles := uniqueFiles(removed)

	return &DiffResult{
		FromCommit: fromCommit,
		ToCommit:   toCommit,
		Added:      added,
		Removed:    removed,
		Summary: DiffSummary{
			FilesAdded:    len(addedFiles),
			FilesRemoved:  len(removedFiles),
			ChunksAdded:   len(added),
			ChunksRemoved: len(removed),
		},
	}, nil
}

// Restore replaces the current active chunks with those from a snapshot.
func Restore(db *sql.DB, snapID int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 현재 활성 청크를 비활성화
	_, err = tx.Exec("UPDATE chunks SET status = 'archived' WHERE status = 'active'")
	if err != nil {
		return fmt.Errorf("archive current: %w", err)
	}

	// 스냅샷의 청크를 다시 활성화 (content_hash 기반 매칭)
	_, err = tx.Exec(`
		UPDATE chunks SET status = 'active'
		WHERE content_hash IN (
			SELECT content_hash FROM snapshot_chunks WHERE snapshot_id = ?
		) AND status = 'archived'
	`, snapID)
	if err != nil {
		return fmt.Errorf("restore chunks: %w", err)
	}

	return tx.Commit()
}

func queryDiffChunks(db *sql.DB, query string, args ...interface{}) ([]ChunkDiff, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var diffs []ChunkDiff
	for rows.Next() {
		var d ChunkDiff
		if err := rows.Scan(&d.File, &d.Heading, &d.ContentHash, &d.FileType); err != nil {
			return nil, err
		}
		diffs = append(diffs, d)
	}
	return diffs, nil
}

func uniqueFiles(diffs []ChunkDiff) map[string]bool {
	files := make(map[string]bool)
	for _, d := range diffs {
		files[d.File] = true
	}
	return files
}
