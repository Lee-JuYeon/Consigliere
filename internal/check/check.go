package check

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/cavss/ledger/internal/store"
)

// Contradiction represents two chunks that may conflict.
type Contradiction struct {
	ChunkA     ChunkInfo
	ChunkB     ChunkInfo
	Similarity float64
	Reason     string
}

type ChunkInfo struct {
	ID       int
	File     string
	Heading  string
	Date     string
	FileType string
	Preview  string
}

type CheckResult struct {
	Contradictions []Contradiction
	Superseded     []SupersededPair
}

// SupersededPair is an older chunk that may be outdated by a newer one.
type SupersededPair struct {
	Older      ChunkInfo
	Newer      ChunkInfo
	Similarity float64
}

// Run performs contradiction detection across all indexed chunks.
// It uses vector similarity (if embeddings exist) and heading similarity.
func Run(db *sql.DB, s *store.Store, threshold float64) (*CheckResult, error) {
	if threshold <= 0 {
		threshold = 0.85
	}

	result := &CheckResult{}

	// 벡터 기반 모순 감지
	vectors, err := s.GetAllVectors()
	if err == nil && len(vectors) > 1 {
		chunkMap, err := loadChunkMap(db)
		if err != nil {
			return nil, err
		}

		// O(n^2) 비교 — 실제 규모에서는 수백~수천 청크이므로 허용
		for i := 0; i < len(vectors); i++ {
			for j := i + 1; j < len(vectors); j++ {
				sim := store.CosineSimilarity(vectors[i].Embedding, vectors[j].Embedding)
				if sim < threshold {
					continue
				}

				a := chunkMap[vectors[i].ChunkID]
				b := chunkMap[vectors[j].ChunkID]

				// 같은 파일 내 청크는 스킵
				if a.File == b.File {
					continue
				}

				// 내용 해시가 같으면 중복이지 모순은 아님
				if a.Preview == b.Preview {
					continue
				}

				// 날짜가 있으면 superseded 판정
				if a.Date != "" && b.Date != "" && a.Date != b.Date {
					older, newer := a, b
					if a.Date > b.Date {
						older, newer = b, a
					}
					result.Superseded = append(result.Superseded, SupersededPair{
						Older:      older,
						Newer:      newer,
						Similarity: sim,
					})
				} else {
					result.Contradictions = append(result.Contradictions, Contradiction{
						ChunkA:     a,
						ChunkB:     b,
						Similarity: sim,
						Reason:     "high similarity, different content across files",
					})
				}
			}
		}
	}

	// 벡터 없으면 헤딩 기반 감지 (같은 헤딩, 다른 파일)
	if len(vectors) == 0 {
		headingDups, err := findHeadingDuplicates(db)
		if err != nil {
			return nil, err
		}
		result.Contradictions = append(result.Contradictions, headingDups...)
	}

	// 유사도 높은 순 정렬
	sort.Slice(result.Contradictions, func(i, j int) bool {
		return result.Contradictions[i].Similarity > result.Contradictions[j].Similarity
	})
	sort.Slice(result.Superseded, func(i, j int) bool {
		return result.Superseded[i].Similarity > result.Superseded[j].Similarity
	})

	return result, nil
}

func loadChunkMap(db *sql.DB) (map[int64]ChunkInfo, error) {
	rows, err := db.Query(
		`SELECT id, file, heading, date, file_type,
		        SUBSTR(content, 1, 200)
		 FROM chunks WHERE status = 'active'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64]ChunkInfo)
	for rows.Next() {
		var c ChunkInfo
		if err := rows.Scan(&c.ID, &c.File, &c.Heading, &c.Date, &c.FileType, &c.Preview); err != nil {
			return nil, err
		}
		m[int64(c.ID)] = c
	}
	return m, nil
}

// findHeadingDuplicates finds chunks with identical headings in different files.
func findHeadingDuplicates(db *sql.DB) ([]Contradiction, error) {
	rows, err := db.Query(`
		SELECT a.id, a.file, a.heading, a.date, a.file_type, SUBSTR(a.content, 1, 200),
		       b.id, b.file, b.heading, b.date, b.file_type, SUBSTR(b.content, 1, 200)
		FROM chunks a
		JOIN chunks b ON a.heading = b.heading AND a.id < b.id AND a.file != b.file
		WHERE a.status = 'active' AND b.status = 'active'
		  AND a.heading != '' AND a.heading IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Contradiction
	for rows.Next() {
		var a, b ChunkInfo
		if err := rows.Scan(&a.ID, &a.File, &a.Heading, &a.Date, &a.FileType, &a.Preview,
			&b.ID, &b.File, &b.Heading, &b.Date, &b.FileType, &b.Preview); err != nil {
			return nil, err
		}
		results = append(results, Contradiction{
			ChunkA:     a,
			ChunkB:     b,
			Similarity: 1.0,
			Reason:     "identical heading in different files",
		})
	}
	return results, nil
}

// FormatFix generates a suggestion for resolving a contradiction.
func FormatFix(c Contradiction, baseDir string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Conflict: %s\n", c.Reason))
	sb.WriteString(fmt.Sprintf("  A: %s — %s\n", c.ChunkA.File, c.ChunkA.Heading))
	sb.WriteString(fmt.Sprintf("  B: %s — %s\n", c.ChunkB.File, c.ChunkB.Heading))
	sb.WriteString("  Suggestion: review both files and consolidate into the authoritative source.\n")
	return sb.String()
}

// FormatSupersededFix generates a suggestion for a superseded pair.
func FormatSupersededFix(sp SupersededPair) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Superseded: newer doc may replace older\n"))
	sb.WriteString(fmt.Sprintf("  Older: %s — %s (date: %s)\n", sp.Older.File, sp.Older.Heading, sp.Older.Date))
	sb.WriteString(fmt.Sprintf("  Newer: %s — %s (date: %s)\n", sp.Newer.File, sp.Newer.Heading, sp.Newer.Date))
	sb.WriteString("  Suggestion: mark older document as superseded or archive it.\n")
	return sb.String()
}
