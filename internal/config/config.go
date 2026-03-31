package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const DefaultConfigFile = ".ledger.toml"

type Config struct {
	Index     IndexConfig     `toml:"index"`
	Search    SearchConfig    `toml:"search"`
	Project   ProjectConfig   `toml:"project"`
	Embedding EmbeddingConfig `toml:"embedding"`
	Plugins   PluginsConfig   `toml:"plugins"`
}

type IndexConfig struct {
	Paths   []string `toml:"paths"`
	Exclude []string `toml:"exclude"`
	Watch   bool     `toml:"watch"`
}

type SearchConfig struct {
	DefaultTopK int     `toml:"default_top_k"`
	MinScore    float64 `toml:"min_score"`
}

type ProjectConfig struct {
	Name string `toml:"name"`
}

type EmbeddingConfig struct {
	Enabled    bool   `toml:"enabled"`
	Model      string `toml:"model"`
	Endpoint   string `toml:"endpoint"`
	Dimensions int    `toml:"dimensions"`
}

type PluginsConfig struct {
	FileTypes  map[string]FileTypeRule `toml:"file_types"`
	TagRules   []TagRule               `toml:"tag_rules"`
	ChunkBy    string                  `toml:"chunk_by"`    // "heading" (default), "paragraph", "separator"
	Separator  string                  `toml:"separator"`   // custom separator for chunk_by=separator
}

type FileTypeRule struct {
	Pattern string `toml:"pattern"` // filename pattern (case-insensitive)
}

type TagRule struct {
	Match string `toml:"match"` // substring to search (case-insensitive)
	Tag   string `toml:"tag"`   // tag to assign
}

func DefaultConfig() Config {
	return Config{
		Index: IndexConfig{
			Paths:   []string{"docs/**/*.md", "CLAUDE.md"},
			Exclude: []string{"docs/memory/MEMORY.md"},
			Watch:   false,
		},
		Search: SearchConfig{
			DefaultTopK: 5,
			MinScore:    0.3,
		},
		Project: ProjectConfig{
			Name: "",
		},
		Embedding: EmbeddingConfig{
			Enabled:    false,
			Model:      "nomic-embed-text",
			Endpoint:   "http://localhost:11434",
			Dimensions: 768,
		},
	}
}

func Load(dir string) (Config, error) {
	cfg := DefaultConfig()
	path := filepath.Join(dir, DefaultConfigFile)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("config read error: %w", err)
	}

	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return cfg, fmt.Errorf("config parse error: %w", err)
	}

	return cfg, nil
}

func (c Config) ResolveFiles(baseDir string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string

	// 제외 패턴을 단순 glob + WalkDir 조합으로 처리
	excludeMap := make(map[string]bool)
	for _, pattern := range c.Index.Exclude {
		resolved, _ := resolvePattern(baseDir, pattern)
		for _, m := range resolved {
			excludeMap[m] = true
		}
	}

	for _, pattern := range c.Index.Paths {
		resolved, err := resolvePattern(baseDir, pattern)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", pattern, err)
		}
		for _, m := range resolved {
			if !excludeMap[m] && !seen[m] {
				seen[m] = true
				files = append(files, m)
			}
		}
	}

	return files, nil
}

// resolvePattern은 ** 재귀 패턴을 지원하는 파일 탐색
func resolvePattern(baseDir, pattern string) ([]string, error) {
	// ** 포함 시 WalkDir로 재귀 탐색
	if strings.Contains(pattern, "**") {
		return resolveDoublestar(baseDir, pattern)
	}
	// 일반 glob
	matches, err := filepath.Glob(filepath.Join(baseDir, pattern))
	return matches, err
}

// resolveDoublestar는 **/*.md 같은 재귀 패턴 처리
func resolveDoublestar(baseDir, pattern string) ([]string, error) {
	// "docs/**/*.md" → dir="docs", ext=".md"
	parts := strings.SplitN(pattern, "**", 2)
	prefix := filepath.Join(baseDir, parts[0]) // "docs/"
	suffix := ""
	if len(parts) > 1 {
		suffix = strings.TrimPrefix(parts[1], "/")
		suffix = strings.TrimPrefix(suffix, string(filepath.Separator))
	}

	// suffix에서 확장자 추출
	ext := ""
	if strings.Contains(suffix, "*") {
		ext = strings.TrimPrefix(suffix, "*")
	}

	var matches []string
	filepath.WalkDir(prefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			return nil
		}
		if ext != "" {
			if strings.HasSuffix(path, ext) {
				matches = append(matches, path)
			}
		} else if suffix == "" {
			matches = append(matches, path)
		}
		return nil
	})

	return matches, nil
}
