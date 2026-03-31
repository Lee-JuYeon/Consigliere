package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cavss/ledger/internal/check"
	"github.com/cavss/ledger/internal/embed"
	"github.com/cavss/ledger/internal/search"
	"github.com/cavss/ledger/internal/store"
)

type Server struct {
	store       *store.Store
	db          *sql.DB
	baseDir     string
	port        int
	embEndpoint string
	embModel    string
}

func NewServer(s *store.Store, baseDir string, port int) *Server {
	return &Server{
		store:       s,
		db:          s.DB(),
		baseDir:     baseDir,
		port:        port,
		embEndpoint: "http://localhost:11434",
		embModel:    "nomic-embed-text",
	}
}

func NewServerWithEmbedding(s *store.Store, baseDir string, port int, endpoint, model string) *Server {
	srv := NewServer(s, baseDir, port)
	if endpoint != "" {
		srv.embEndpoint = endpoint
	}
	if model != "" {
		srv.embModel = model
	}
	return srv
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/search", s.cors(s.HandleSearch))
	mux.HandleFunc("/api/status", s.cors(s.HandleStatus))
	mux.HandleFunc("/api/chunks", s.cors(s.HandleChunks))
	mux.HandleFunc("/api/check", s.cors(s.HandleCheck))
	mux.HandleFunc("/api/health", s.cors(s.HandleHealth))

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("Ledger API server listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) cors(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(200)
			return
		}
		h(w, r)
	}
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// GET /api/search?q=query&top=5&type=changelog&tag=security&mode=keyword&role=developer
func (s *Server) HandleSearch(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	q := r.URL.Query()
	query := q.Get("q")
	if query == "" {
		writeError(w, 400, "missing q parameter")
		return
	}

	topK := 5
	if v := q.Get("top"); v != "" {
		topK, _ = strconv.Atoi(v)
	}

	mode := q.Get("mode")
	if mode == "" {
		mode = "keyword"
	}

	var filters []search.Filter
	if v := q.Get("type"); v != "" {
		filters = append(filters, search.FileTypeFilter{FileType: v})
	}
	if v := q.Get("tag"); v != "" {
		filters = append(filters, search.TagFilter{Tag: v})
	}
	if v := q.Get("after"); v != "" {
		filters = append(filters, search.DateAfterFilter{Date: v})
	}
	if v := q.Get("before"); v != "" {
		filters = append(filters, search.DateBeforeFilter{Date: v})
	}

	searcher := search.NewWithStore(s.store)
	var results []search.Result
	var err error
	actualMode := mode

	switch mode {
	case "semantic", "hybrid":
		// Ollama 임베딩 시도, 실패하면 BM25 폴백
		client := embed.NewClient(s.embEndpoint, s.embModel)
		queryVec, embErr := client.Embed(query)
		if embErr != nil {
			// 폴백
			actualMode = "keyword (fallback from " + mode + ")"
			results, err = searcher.Search(query, topK, filters...)
		} else if mode == "semantic" {
			results, err = searcher.SemanticSearch(queryVec, topK, filters...)
		} else {
			results, err = searcher.HybridSearch(query, queryVec, topK, 0.5, filters...)
		}
	default:
		results, err = searcher.Search(query, topK, filters...)
		actualMode = "keyword"
	}

	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	role := q.Get("role")
	if role != "" {
		results = search.ApplyRoleBoost(results, role)
	}

	type apiResult struct {
		ID        int     `json:"id"`
		File      string  `json:"file"`
		Heading   string  `json:"heading"`
		Content   string  `json:"content"`
		FileType  string  `json:"file_type"`
		Date      string  `json:"date"`
		Tags      string  `json:"tags"`
		LineStart int     `json:"line_start"`
		LineEnd   int     `json:"line_end"`
		Score     float64 `json:"score"`
	}

	var out []apiResult
	for _, r := range results {
		out = append(out, apiResult{
			ID: r.ID, File: r.File, Heading: r.Heading,
			Content: r.Content, FileType: r.FileType, Date: r.Date,
			Tags: r.Tags, LineStart: r.LineStart, LineEnd: r.LineEnd,
			Score: r.Score,
		})
	}

	elapsed := time.Since(start)

	writeJSON(w, map[string]interface{}{
		"query":   query,
		"results": out,
		"total":   len(out),
		"meta": map[string]interface{}{
			"mode":       actualMode,
			"role":       role,
			"top_k":      topK,
			"elapsed_ms": elapsed.Milliseconds(),
		},
	})
}

// GET /api/status
func (s *Server) HandleStatus(w http.ResponseWriter, r *http.Request) {
	chunks, files, _ := s.store.GetStats()
	vecCount, vecModel := s.store.GetVectorStats()
	writeJSON(w, map[string]interface{}{
		"files_indexed": files,
		"total_chunks":  chunks,
		"embeddings":    vecCount,
		"embed_model":   vecModel,
	})
}

// GET /api/chunks?file=path&type=changelog&limit=50&offset=0
func (s *Server) HandleChunks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 50
	offset := 0
	if v := q.Get("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	if v := q.Get("offset"); v != "" {
		offset, _ = strconv.Atoi(v)
	}

	query := "SELECT id, file, heading, content, file_type, date, tags, line_start, line_end FROM chunks WHERE status = 'active'"
	var args []interface{}

	if v := q.Get("file"); v != "" {
		query += " AND file LIKE ?"
		args = append(args, "%"+v+"%")
	}
	if v := q.Get("type"); v != "" {
		query += " AND file_type = ?"
		args = append(args, v)
	}

	query += " ORDER BY file, line_start LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer rows.Close()

	type chunk struct {
		ID        int    `json:"id"`
		File      string `json:"file"`
		Heading   string `json:"heading"`
		Content   string `json:"content"`
		FileType  string `json:"file_type"`
		Date      string `json:"date"`
		Tags      string `json:"tags"`
		LineStart int    `json:"line_start"`
		LineEnd   int    `json:"line_end"`
	}

	var out []chunk
	for rows.Next() {
		var c chunk
		rows.Scan(&c.ID, &c.File, &c.Heading, &c.Content, &c.FileType, &c.Date, &c.Tags, &c.LineStart, &c.LineEnd)
		out = append(out, c)
	}

	writeJSON(w, map[string]interface{}{
		"chunks": out,
		"total":  len(out),
	})
}

// GET /api/check?threshold=0.9
func (s *Server) HandleCheck(w http.ResponseWriter, r *http.Request) {
	threshold := 0.85
	if v := r.URL.Query().Get("threshold"); v != "" {
		fmt.Sscanf(v, "%f", &threshold)
	}

	result, err := check.Run(s.db, s.store, threshold)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	type contradiction struct {
		FileA      string  `json:"file_a"`
		HeadingA   string  `json:"heading_a"`
		FileB      string  `json:"file_b"`
		HeadingB   string  `json:"heading_b"`
		Similarity float64 `json:"similarity"`
		Reason     string  `json:"reason"`
	}

	var contrs []contradiction
	for _, c := range result.Contradictions {
		contrs = append(contrs, contradiction{
			FileA: c.ChunkA.File, HeadingA: c.ChunkA.Heading,
			FileB: c.ChunkB.File, HeadingB: c.ChunkB.Heading,
			Similarity: c.Similarity, Reason: c.Reason,
		})
	}

	type superseded struct {
		OlderFile    string  `json:"older_file"`
		OlderHeading string  `json:"older_heading"`
		OlderDate    string  `json:"older_date"`
		NewerFile    string  `json:"newer_file"`
		NewerHeading string  `json:"newer_heading"`
		NewerDate    string  `json:"newer_date"`
		Similarity   float64 `json:"similarity"`
	}

	var sups []superseded
	for _, sp := range result.Superseded {
		sups = append(sups, superseded{
			OlderFile: sp.Older.File, OlderHeading: sp.Older.Heading, OlderDate: sp.Older.Date,
			NewerFile: sp.Newer.File, NewerHeading: sp.Newer.Heading, NewerDate: sp.Newer.Date,
			Similarity: sp.Similarity,
		})
	}

	writeJSON(w, map[string]interface{}{
		"contradictions": contrs,
		"superseded":     sups,
		"total":          len(contrs) + len(sups),
	})
}

// GET /api/health
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"status":  "ok",
		"version": "0.2.0",
	})
}

// ParsePort extracts port from args or returns default.
func ParsePort(args []string, defaultPort int) int {
	for i, arg := range args {
		if (arg == "--port" || arg == "-p") && i+1 < len(args) {
			if p, err := strconv.Atoi(args[i+1]); err == nil {
				return p
			}
		}
		if strings.HasPrefix(arg, "--port=") {
			if p, err := strconv.Atoi(strings.TrimPrefix(arg, "--port=")); err == nil {
				return p
			}
		}
	}
	return defaultPort
}
