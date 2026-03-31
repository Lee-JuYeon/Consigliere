package store

import (
	"crypto/md5"
	"fmt"
	"os"
	"time"

	"github.com/cavss/ledger/internal/docpipe"
)

func (s *Store) UpsertChunks(chunks []docpipe.Chunk) (inserted, updated, skipped int, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, c := range chunks {
		var existingID int
		var existingHash string
		err := tx.QueryRow(
			"SELECT id, content_hash FROM chunks WHERE file = ? AND line_start = ? AND status = 'active'",
			c.File, c.LineStart,
		).Scan(&existingID, &existingHash)

		if err == nil {
			// 기존 청크 존재
			if existingHash == c.ContentHash {
				skipped++
				continue
			}
			// 변경됨 → 업데이트
			_, err = tx.Exec(
				`UPDATE chunks SET heading=?, content=?, content_hash=?, file_type=?, date=?, tags=?, line_end=?, updated_at=?
				 WHERE id=?`,
				c.Heading, c.Content, c.ContentHash, c.FileType, c.Date, c.Tags, c.LineEnd, time.Now(), existingID,
			)
			if err != nil {
				return 0, 0, 0, fmt.Errorf("update chunk: %w", err)
			}
			// FTS 동기화
			tx.Exec("DELETE FROM chunks_fts WHERE rowid = ?", existingID)
			tx.Exec("INSERT INTO chunks_fts(rowid, content, heading, tags) VALUES (?, ?, ?, ?)",
				existingID, c.Content, c.Heading, c.Tags)
			updated++
		} else {
			// 새 청크 삽입
			res, err := tx.Exec(
				`INSERT INTO chunks (file, heading, content, content_hash, file_type, date, tags, line_start, line_end)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				c.File, c.Heading, c.Content, c.ContentHash, c.FileType, c.Date, c.Tags, c.LineStart, c.LineEnd,
			)
			if err != nil {
				return 0, 0, 0, fmt.Errorf("insert chunk: %w", err)
			}
			id, _ := res.LastInsertId()
			tx.Exec("INSERT INTO chunks_fts(rowid, content, heading, tags) VALUES (?, ?, ?, ?)",
				id, c.Content, c.Heading, c.Tags)
			inserted++
		}
	}

	return inserted, updated, skipped, tx.Commit()
}

func (s *Store) UpdateMetadata(filePath string, fileType string, chunkCount int) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	hash := fmt.Sprintf("%x", md5.Sum(data))
	info, _ := os.Stat(filePath)
	var modTime time.Time
	if info != nil {
		modTime = info.ModTime()
	}

	_, err = s.db.Exec(
		`INSERT INTO metadata (file, file_hash, file_type, chunk_count, last_modified, last_indexed)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(file) DO UPDATE SET
		   file_hash=excluded.file_hash,
		   file_type=excluded.file_type,
		   chunk_count=excluded.chunk_count,
		   last_modified=excluded.last_modified,
		   last_indexed=excluded.last_indexed`,
		filePath, hash, fileType, chunkCount, modTime, time.Now(),
	)
	return err
}

func (s *Store) IsFileChanged(filePath string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return true, nil
	}
	hash := fmt.Sprintf("%x", md5.Sum(data))

	var storedHash string
	err = s.db.QueryRow("SELECT file_hash FROM metadata WHERE file = ?", filePath).Scan(&storedHash)
	if err != nil {
		return true, nil // 메타데이터 없음 = 신규 파일
	}

	return hash != storedHash, nil
}

func (s *Store) RemoveChunksForFile(filePath string) error {
	_, err := s.db.Exec("DELETE FROM chunks WHERE file = ?", filePath)
	return err
}

func (s *Store) GetStats() (totalChunks, totalFiles int, err error) {
	s.db.QueryRow("SELECT COUNT(*) FROM chunks WHERE status = 'active'").Scan(&totalChunks)
	s.db.QueryRow("SELECT COUNT(*) FROM metadata").Scan(&totalFiles)
	return totalChunks, totalFiles, nil
}
