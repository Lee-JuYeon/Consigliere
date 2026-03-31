package store

import (
	"encoding/binary"
	"fmt"
	"math"
)

// StoreVector saves an embedding vector for a chunk.
func (s *Store) StoreVector(chunkID int64, embedding []float32, model string) error {
	blob := encodeVector(embedding)
	_, err := s.db.Exec(
		`INSERT INTO vectors (chunk_id, embedding, model)
		 VALUES (?, ?, ?)
		 ON CONFLICT(chunk_id) DO UPDATE SET
		   embedding=excluded.embedding,
		   model=excluded.model,
		   created_at=CURRENT_TIMESTAMP`,
		chunkID, blob, model,
	)
	return err
}

// GetVector retrieves the embedding for a chunk.
func (s *Store) GetVector(chunkID int64) ([]float32, error) {
	var blob []byte
	err := s.db.QueryRow("SELECT embedding FROM vectors WHERE chunk_id = ?", chunkID).Scan(&blob)
	if err != nil {
		return nil, err
	}
	return decodeVector(blob), nil
}

// VectorEntry holds a chunk ID and its embedding.
type VectorEntry struct {
	ChunkID   int64
	Embedding []float32
}

// GetAllVectors loads all stored vectors into memory for similarity search.
func (s *Store) GetAllVectors() ([]VectorEntry, error) {
	rows, err := s.db.Query("SELECT chunk_id, embedding FROM vectors")
	if err != nil {
		return nil, fmt.Errorf("query vectors: %w", err)
	}
	defer rows.Close()

	var entries []VectorEntry
	for rows.Next() {
		var e VectorEntry
		var blob []byte
		if err := rows.Scan(&e.ChunkID, &blob); err != nil {
			return nil, fmt.Errorf("scan vector: %w", err)
		}
		e.Embedding = decodeVector(blob)
		entries = append(entries, e)
	}
	return entries, nil
}

// RemoveVectorsForFile removes vectors for all chunks belonging to a file.
func (s *Store) RemoveVectorsForFile(filePath string) error {
	_, err := s.db.Exec(
		"DELETE FROM vectors WHERE chunk_id IN (SELECT id FROM chunks WHERE file = ?)",
		filePath,
	)
	return err
}

// GetVectorStats returns the number of stored vectors.
func (s *Store) GetVectorStats() (int, string) {
	var count int
	var model string
	s.db.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&count)
	s.db.QueryRow("SELECT model FROM vectors LIMIT 1").Scan(&model)
	return count, model
}

// HasVector checks if a chunk already has a stored vector.
func (s *Store) HasVector(chunkID int64) bool {
	var exists int
	s.db.QueryRow("SELECT 1 FROM vectors WHERE chunk_id = ?", chunkID).Scan(&exists)
	return exists == 1
}

// GetChunksWithoutVectors returns chunk IDs that don't have embeddings yet.
func (s *Store) GetChunksWithoutVectors() ([]int64, error) {
	rows, err := s.db.Query(
		`SELECT c.id FROM chunks c
		 LEFT JOIN vectors v ON c.id = v.chunk_id
		 WHERE v.chunk_id IS NULL AND c.status = 'active'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// GetChunkContent returns the content for a given chunk ID.
func (s *Store) GetChunkContent(chunkID int64) (string, error) {
	var content string
	err := s.db.QueryRow("SELECT content FROM chunks WHERE id = ?", chunkID).Scan(&content)
	return content, err
}

// CosineSimilarity computes cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

func encodeVector(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func decodeVector(buf []byte) []float32 {
	v := make([]float32, len(buf)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return v
}
