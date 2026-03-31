package search

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/cavss/ledger/internal/store"
)

type Result struct {
	ID        int
	File      string
	Heading   string
	Content   string
	FileType  string
	Date      string
	Tags      string
	LineStart int
	LineEnd   int
	Score     float64
}

type Searcher struct {
	db    *sql.DB
	store *store.Store
}

func New(db *sql.DB) *Searcher {
	return &Searcher{db: db}
}

func NewWithStore(s *store.Store) *Searcher {
	return &Searcher{db: s.DB(), store: s}
}

// FTS5 기반 BM25 검색
func (s *Searcher) Search(query string, topK int, filters ...Filter) ([]Result, error) {
	if topK <= 0 {
		topK = 5
	}

	// FTS5 BM25 검색
	baseSQL := `
		SELECT c.id, c.file, c.heading, c.content, c.file_type, c.date, c.tags,
		       c.line_start, c.line_end, rank
		FROM chunks_fts fts
		JOIN chunks c ON c.id = fts.rowid
		WHERE chunks_fts MATCH ?
		  AND c.status = 'active'
	`

	args := []interface{}{buildFTSQuery(query)}

	// 필터 적용
	for _, f := range filters {
		clause, arg := f.ToSQL()
		if clause != "" {
			baseSQL += " AND " + clause
			if arg != nil {
				args = append(args, arg)
			}
		}
	}

	baseSQL += " ORDER BY rank LIMIT ?"
	args = append(args, topK)

	rows, err := s.db.Query(baseSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		var rank float64
		if err := rows.Scan(&r.ID, &r.File, &r.Heading, &r.Content, &r.FileType,
			&r.Date, &r.Tags, &r.LineStart, &r.LineEnd, &rank); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		// FTS5 rank는 음수 (낮을수록 좋음), 양수 스코어로 변환
		r.Score = -rank
		results = append(results, r)
	}

	return results, nil
}

// FTS5 쿼리 구성: "보안 관련 결정" → "보안 OR 관련 OR 결정"
func buildFTSQuery(query string) string {
	words := strings.Fields(query)
	if len(words) == 0 {
		return query
	}

	// 각 단어를 쌍따옴표로 감싸서 특수문자 이스케이프
	var quoted []string
	for _, w := range words {
		w = strings.ReplaceAll(w, "\"", "")
		if w != "" {
			quoted = append(quoted, fmt.Sprintf("\"%s\"", w))
		}
	}

	return strings.Join(quoted, " OR ")
}

// 필터 인터페이스
type Filter interface {
	ToSQL() (string, interface{})
}

type FileTypeFilter struct {
	FileType string
}

func (f FileTypeFilter) ToSQL() (string, interface{}) {
	return "c.file_type = ?", f.FileType
}

type DateAfterFilter struct {
	Date string
}

func (f DateAfterFilter) ToSQL() (string, interface{}) {
	return "c.date >= ?", f.Date
}

type DateBeforeFilter struct {
	Date string
}

func (f DateBeforeFilter) ToSQL() (string, interface{}) {
	return "c.date <= ?", f.Date
}

type TagFilter struct {
	Tag string
}

func (f TagFilter) ToSQL() (string, interface{}) {
	return "c.tags LIKE ?", fmt.Sprintf("%%%s%%", f.Tag)
}

// RoleWeights maps roles to file-type boost multipliers.
// CLI Company 16개 에이전트 + 범용 역할 전체 지원.
var RoleWeights = map[string]map[string]float64{
	// === Director / 경영 ===
	"director": {
		"dashboard": 1.5, "workflow": 1.4, "rules": 1.3, "changelog": 1.3,
		"issues": 1.1, "debate": 1.2, "memory": 1.1, "claude": 0.8,
		"checklist": 0.9, "other": 1.0,
	},
	"pm": {
		"dashboard": 1.5, "workflow": 1.4, "issues": 1.3, "changelog": 1.2,
		"rules": 1.1, "debate": 1.2, "checklist": 1.1, "memory": 1.0,
		"claude": 0.8, "other": 1.0,
	},

	// === Architecture ===
	"system_architect": {
		"changelog": 1.5, "rules": 1.4, "workflow": 1.3, "debate": 1.3,
		"issues": 1.2, "dashboard": 1.0, "claude": 1.1, "checklist": 0.9,
		"memory": 0.9, "other": 1.0,
	},
	"data_architect": {
		"changelog": 1.5, "rules": 1.3, "issues": 1.3, "debate": 1.2,
		"workflow": 1.1, "claude": 1.1, "dashboard": 0.9, "checklist": 0.9,
		"memory": 0.9, "other": 1.0,
	},
	"cloud_architect": {
		"rules": 1.5, "changelog": 1.4, "issues": 1.2, "workflow": 1.2,
		"debate": 1.2, "claude": 1.0, "dashboard": 0.9, "checklist": 1.0,
		"memory": 0.8, "other": 1.0,
	},

	// === Backend ===
	"api_developer": {
		"changelog": 1.5, "issues": 1.4, "rules": 1.2, "claude": 1.3,
		"debate": 1.1, "workflow": 1.0, "checklist": 1.0, "dashboard": 0.8,
		"memory": 0.8, "other": 1.0,
	},
	"business_logic_developer": {
		"changelog": 1.5, "issues": 1.4, "rules": 1.3, "claude": 1.2,
		"debate": 1.1, "workflow": 1.0, "checklist": 1.0, "dashboard": 0.8,
		"memory": 0.8, "other": 1.0,
	},
	"database_developer": {
		"changelog": 1.5, "rules": 1.4, "issues": 1.3, "claude": 1.2,
		"debate": 1.1, "workflow": 1.0, "checklist": 1.0, "dashboard": 0.8,
		"memory": 0.8, "other": 1.0,
	},

	// === Frontend ===
	"web_developer": {
		"changelog": 1.5, "issues": 1.3, "rules": 1.3, "claude": 1.4,
		"checklist": 1.1, "debate": 1.0, "workflow": 1.0, "dashboard": 0.8,
		"memory": 0.8, "other": 1.0,
	},
	"ios_developer": {
		"changelog": 1.5, "rules": 1.4, "claude": 1.4, "issues": 1.3,
		"checklist": 1.2, "debate": 1.1, "workflow": 1.0, "dashboard": 0.7,
		"memory": 0.8, "other": 1.0,
	},
	"android_developer": {
		"changelog": 1.5, "rules": 1.4, "claude": 1.4, "issues": 1.3,
		"checklist": 1.2, "debate": 1.1, "workflow": 1.0, "dashboard": 0.7,
		"memory": 0.8, "other": 1.0,
	},
	"ui_ux_designer": {
		"rules": 1.4, "issues": 1.3, "changelog": 1.3, "claude": 1.2,
		"workflow": 1.1, "debate": 1.0, "checklist": 1.0, "dashboard": 1.0,
		"memory": 0.8, "other": 1.0,
	},

	// === Security ===
	"app_security": {
		"rules": 1.5, "issues": 1.5, "changelog": 1.3, "debate": 1.3,
		"claude": 1.0, "workflow": 1.0, "checklist": 1.1, "dashboard": 0.9,
		"memory": 0.8, "other": 1.0,
	},
	"db_security": {
		"rules": 1.5, "issues": 1.5, "changelog": 1.2, "debate": 1.2,
		"claude": 1.0, "workflow": 1.0, "checklist": 1.1, "dashboard": 0.8,
		"memory": 0.8, "other": 1.0,
	},
	"infra_security": {
		"rules": 1.5, "issues": 1.4, "changelog": 1.2, "debate": 1.2,
		"workflow": 1.1, "claude": 1.0, "checklist": 1.1, "dashboard": 0.8,
		"memory": 0.8, "other": 1.0,
	},
	"security_researcher": {
		"issues": 1.5, "rules": 1.4, "debate": 1.3, "changelog": 1.2,
		"claude": 1.0, "workflow": 1.0, "checklist": 0.9, "dashboard": 0.8,
		"memory": 0.9, "other": 1.0,
	},

	// === QA ===
	"functional_tester": {
		"issues": 1.5, "changelog": 1.4, "checklist": 1.3, "rules": 1.2,
		"debate": 1.1, "workflow": 1.1, "claude": 0.9, "dashboard": 1.0,
		"memory": 0.8, "other": 1.0,
	},
	"performance_tester": {
		"issues": 1.5, "changelog": 1.3, "checklist": 1.3, "rules": 1.2,
		"debate": 1.0, "workflow": 1.1, "claude": 0.9, "dashboard": 1.1,
		"memory": 0.8, "other": 1.0,
	},

	// === DevOps ===
	"cicd_engineer": {
		"workflow": 1.5, "rules": 1.3, "changelog": 1.3, "issues": 1.2,
		"checklist": 1.2, "debate": 1.0, "claude": 1.0, "dashboard": 1.0,
		"memory": 0.8, "other": 1.0,
	},
	"infra_engineer": {
		"rules": 1.4, "workflow": 1.4, "changelog": 1.3, "issues": 1.2,
		"checklist": 1.2, "debate": 1.0, "claude": 1.0, "dashboard": 1.0,
		"memory": 0.8, "other": 1.0,
	},

	// === Support ===
	"tech_writer": {
		"changelog": 1.5, "rules": 1.3, "workflow": 1.3, "debate": 1.2,
		"issues": 1.1, "dashboard": 1.1, "claude": 1.0, "checklist": 1.0,
		"memory": 1.0, "other": 1.0,
	},
	"data_analyst": {
		"dashboard": 1.5, "issues": 1.3, "changelog": 1.3, "rules": 1.1,
		"workflow": 1.1, "debate": 1.0, "claude": 0.9, "checklist": 0.9,
		"memory": 1.0, "other": 1.0,
	},
	"tech_researcher": {
		"debate": 1.4, "changelog": 1.3, "issues": 1.3, "rules": 1.2,
		"workflow": 1.0, "claude": 1.0, "dashboard": 1.0, "checklist": 0.9,
		"memory": 1.0, "other": 1.0,
	},
	"market_researcher": {
		"dashboard": 1.4, "debate": 1.3, "changelog": 1.2, "rules": 1.1,
		"issues": 1.1, "workflow": 1.0, "claude": 0.9, "checklist": 0.8,
		"memory": 1.0, "other": 1.0,
	},
	"accountant": {
		"dashboard": 1.5, "rules": 1.4, "changelog": 1.2, "workflow": 1.1,
		"issues": 1.0, "debate": 1.0, "claude": 0.8, "checklist": 1.0,
		"memory": 0.9, "other": 1.0,
	},
	"growth_marketer": {
		"dashboard": 1.4, "changelog": 1.3, "debate": 1.2, "rules": 1.1,
		"issues": 1.1, "workflow": 1.0, "claude": 0.9, "checklist": 0.9,
		"memory": 1.0, "other": 1.0,
	},
	"content_marketer": {
		"changelog": 1.4, "dashboard": 1.3, "debate": 1.2, "rules": 1.1,
		"issues": 1.0, "workflow": 1.0, "claude": 0.9, "checklist": 0.9,
		"memory": 1.0, "other": 1.0,
	},

	// === 범용 폴백 ===
	"developer": {
		"changelog": 1.5, "issues": 1.3, "rules": 1.2, "claude": 1.4,
		"workflow": 1.1, "debate": 1.0, "checklist": 1.0, "dashboard": 0.8,
		"memory": 0.9, "other": 1.0,
	},
	"security": {
		"rules": 1.5, "issues": 1.4, "changelog": 1.2, "debate": 1.2,
		"claude": 1.0, "workflow": 1.1, "checklist": 1.1, "dashboard": 0.9,
		"memory": 0.8, "other": 1.0,
	},
	"qa": {
		"issues": 1.5, "changelog": 1.4, "checklist": 1.3, "rules": 1.2,
		"debate": 1.1, "workflow": 1.3, "claude": 0.9, "dashboard": 1.1,
		"memory": 0.8, "other": 1.0,
	},
}

// ApplyRoleBoost adjusts result scores based on role weights.
func ApplyRoleBoost(results []Result, role string) []Result {
	weights, ok := RoleWeights[role]
	if !ok {
		return results
	}

	for i := range results {
		boost, exists := weights[results[i].FileType]
		if !exists {
			boost = 1.0
		}
		results[i].Score *= boost
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// SemanticSearch finds similar chunks using cosine similarity on embeddings.
func (s *Searcher) SemanticSearch(queryVec []float32, topK int, filters ...Filter) ([]Result, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store required for semantic search")
	}
	if topK <= 0 {
		topK = 5
	}

	vectors, err := s.store.GetAllVectors()
	if err != nil {
		return nil, fmt.Errorf("load vectors: %w", err)
	}

	if len(vectors) == 0 {
		return nil, nil
	}

	// 유사도 계산
	type scored struct {
		chunkID int64
		score   float64
	}
	var scored_ []scored
	for _, v := range vectors {
		sim := store.CosineSimilarity(queryVec, v.Embedding)
		if sim > 0 {
			scored_ = append(scored_, scored{chunkID: v.ChunkID, score: sim})
		}
	}

	sort.Slice(scored_, func(i, j int) bool {
		return scored_[i].score > scored_[j].score
	})

	// 상위 후보에서 필터링 후 결과 조회
	var results []Result
	for _, sc := range scored_ {
		if len(results) >= topK {
			break
		}
		r, err := s.getChunkResult(sc.chunkID, filters...)
		if err != nil {
			continue
		}
		r.Score = sc.score
		results = append(results, r)
	}

	return results, nil
}

// HybridSearch combines BM25 keyword search and semantic search.
// alpha: 0.0 = pure keyword, 1.0 = pure semantic
func (s *Searcher) HybridSearch(query string, queryVec []float32, topK int, alpha float64, filters ...Filter) ([]Result, error) {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}

	// BM25 검색 (더 많은 후보)
	candidateK := topK * 3
	if candidateK < 20 {
		candidateK = 20
	}

	bm25Results, err := s.Search(query, candidateK, filters...)
	if err != nil {
		return nil, fmt.Errorf("bm25 search: %w", err)
	}

	// 시맨틱 검색
	semResults, err := s.SemanticSearch(queryVec, candidateK, filters...)
	if err != nil {
		return nil, fmt.Errorf("semantic search: %w", err)
	}

	// 점수 정규화 + 통합
	scoreMap := make(map[int]float64) // chunk ID → combined score
	resultMap := make(map[int]Result)

	// BM25 점수 정규화
	var maxBM25 float64
	for _, r := range bm25Results {
		if r.Score > maxBM25 {
			maxBM25 = r.Score
		}
	}
	if maxBM25 > 0 {
		for _, r := range bm25Results {
			normalized := r.Score / maxBM25
			scoreMap[r.ID] += (1 - alpha) * normalized
			resultMap[r.ID] = r
		}
	}

	// 시맨틱 점수는 이미 0~1 범위 (cosine similarity)
	for _, r := range semResults {
		scoreMap[r.ID] += alpha * r.Score
		if _, exists := resultMap[r.ID]; !exists {
			resultMap[r.ID] = r
		}
	}

	// 점수순 정렬
	type idScore struct {
		id    int
		score float64
	}
	var ranked []idScore
	for id, score := range scoreMap {
		ranked = append(ranked, idScore{id, score})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	var results []Result
	for i, r := range ranked {
		if i >= topK {
			break
		}
		result := resultMap[r.id]
		result.Score = r.score
		results = append(results, result)
	}

	return results, nil
}

func (s *Searcher) getChunkResult(chunkID int64, filters ...Filter) (Result, error) {
	baseSQL := `SELECT id, file, heading, content, file_type, date, tags, line_start, line_end
		FROM chunks WHERE id = ? AND status = 'active'`
	args := []interface{}{chunkID}

	for _, f := range filters {
		clause, arg := f.ToSQL()
		if clause != "" {
			baseSQL += " AND " + clause
			if arg != nil {
				args = append(args, arg)
			}
		}
	}

	var r Result
	err := s.db.QueryRow(baseSQL, args...).Scan(
		&r.ID, &r.File, &r.Heading, &r.Content, &r.FileType,
		&r.Date, &r.Tags, &r.LineStart, &r.LineEnd,
	)
	return r, err
}
