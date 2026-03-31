package docpipe

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Chunk struct {
	File        string
	Heading     string
	Content     string
	ContentHash string
	FileType    string
	Date        string
	Tags        string
	LineStart   int
	LineEnd     int
}

var headingRegex = regexp.MustCompile(`^#{1,6}\s+`)
var dateRegex = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// ParseOptions allows external config to influence parsing.
type ParseOptions struct {
	CustomFileTypes map[string]string // pattern → type name
	CustomTagRules  []struct{ Match, Tag string }
	ChunkBy         string // "heading", "paragraph", "separator"
	Separator       string // custom separator string
}

func ParseFile(path string) ([]Chunk, error) {
	return ParseFileWithOptions(path, ParseOptions{})
}

func ParseFileWithOptions(path string, opts ParseOptions) ([]Chunk, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	fileType := detectFileTypeWithCustom(path, opts.CustomFileTypes)
	lines := strings.Split(string(data), "\n")

	var chunks []Chunk
	switch opts.ChunkBy {
	case "paragraph":
		chunks = splitByParagraphs(lines, path, fileType)
	case "separator":
		sep := opts.Separator
		if sep == "" {
			sep = "---"
		}
		chunks = splitBySeparator(lines, path, fileType, sep)
	default:
		chunks = splitByHeadings(lines, path, fileType)
	}

	// 커스텀 태그 룰 적용
	if len(opts.CustomTagRules) > 0 {
		for i := range chunks {
			chunks[i].Tags = applyCustomTags(chunks[i].Content, chunks[i].Tags, opts.CustomTagRules)
		}
	}

	return chunks, nil
}

func detectFileTypeWithCustom(path string, custom map[string]string) string {
	base := strings.ToUpper(filepath.Base(path))
	// 커스텀 룰 먼저 체크
	for pattern, typeName := range custom {
		if strings.Contains(base, strings.ToUpper(pattern)) {
			return typeName
		}
	}
	return detectFileType(path)
}

func detectFileType(path string) string {
	base := strings.ToUpper(filepath.Base(path))
	switch {
	case strings.Contains(base, "CHANGELOG"):
		return "changelog"
	case strings.Contains(base, "ISSUES"):
		return "issues"
	case strings.Contains(base, "RULES"):
		return "rules"
	case strings.Contains(base, "DASHBOARD"):
		return "dashboard"
	case strings.Contains(base, "WORKFLOW"):
		return "workflow"
	case strings.Contains(base, "CLAUDE"):
		return "claude"
	case strings.Contains(base, "MEMORY"):
		return "memory"
	default:
		return "other"
	}
}

func splitByHeadings(lines []string, filePath string, fileType string) []Chunk {
	var chunks []Chunk
	var currentHeading string
	var currentLines []string
	var startLine int

	flush := func(endLine int) {
		if len(currentLines) == 0 {
			return
		}
		content := strings.TrimSpace(strings.Join(currentLines, "\n"))
		if content == "" {
			return
		}
		hash := fmt.Sprintf("%x", md5.Sum([]byte(content)))
		date := extractDate(content)
		tags := extractTags(content, fileType)

		chunks = append(chunks, Chunk{
			File:        filePath,
			Heading:     currentHeading,
			Content:     content,
			ContentHash: hash,
			FileType:    fileType,
			Date:        date,
			Tags:        tags,
			LineStart:   startLine,
			LineEnd:     endLine,
		})
	}

	for i, line := range lines {
		if headingRegex.MatchString(line) {
			flush(i)
			currentHeading = strings.TrimSpace(line)
			currentLines = []string{line}
			startLine = i + 1
		} else {
			currentLines = append(currentLines, line)
		}
	}
	flush(len(lines))

	// 헤딩이 없는 파일은 전체를 하나의 청크로
	if len(chunks) == 0 && len(lines) > 0 {
		content := strings.TrimSpace(strings.Join(lines, "\n"))
		if content != "" {
			hash := fmt.Sprintf("%x", md5.Sum([]byte(content)))
			chunks = append(chunks, Chunk{
				File:        filePath,
				Heading:     filepath.Base(filePath),
				Content:     content,
				ContentHash: hash,
				FileType:    fileType,
				Date:        extractDate(content),
				Tags:        extractTags(content, fileType),
				LineStart:   1,
				LineEnd:     len(lines),
			})
		}
	}

	return chunks
}

func extractDate(content string) string {
	match := dateRegex.FindString(content)
	if match != "" {
		if _, err := time.Parse("2006-01-02", match); err == nil {
			return match
		}
	}
	return ""
}

func splitByParagraphs(lines []string, filePath string, fileType string) []Chunk {
	var chunks []Chunk
	var currentLines []string
	startLine := 1

	flush := func(endLine int) {
		content := strings.TrimSpace(strings.Join(currentLines, "\n"))
		if content == "" {
			return
		}
		hash := fmt.Sprintf("%x", md5.Sum([]byte(content)))
		heading := ""
		for _, line := range currentLines {
			if headingRegex.MatchString(line) {
				heading = strings.TrimSpace(line)
				break
			}
		}
		chunks = append(chunks, Chunk{
			File: filePath, Heading: heading, Content: content,
			ContentHash: hash, FileType: fileType,
			Date: extractDate(content), Tags: extractTags(content, fileType),
			LineStart: startLine, LineEnd: endLine,
		})
	}

	for i, line := range lines {
		if strings.TrimSpace(line) == "" && len(currentLines) > 0 {
			flush(i)
			currentLines = nil
			startLine = i + 2
		} else {
			currentLines = append(currentLines, line)
		}
	}
	flush(len(lines))
	return chunks
}

func splitBySeparator(lines []string, filePath string, fileType string, separator string) []Chunk {
	var chunks []Chunk
	var currentLines []string
	startLine := 1

	flush := func(endLine int) {
		content := strings.TrimSpace(strings.Join(currentLines, "\n"))
		if content == "" {
			return
		}
		hash := fmt.Sprintf("%x", md5.Sum([]byte(content)))
		heading := ""
		for _, line := range currentLines {
			if headingRegex.MatchString(line) {
				heading = strings.TrimSpace(line)
				break
			}
		}
		chunks = append(chunks, Chunk{
			File: filePath, Heading: heading, Content: content,
			ContentHash: hash, FileType: fileType,
			Date: extractDate(content), Tags: extractTags(content, fileType),
			LineStart: startLine, LineEnd: endLine,
		})
	}

	for i, line := range lines {
		if strings.TrimSpace(line) == separator {
			flush(i)
			currentLines = nil
			startLine = i + 2
		} else {
			currentLines = append(currentLines, line)
		}
	}
	flush(len(lines))
	return chunks
}

func applyCustomTags(content, existingTags string, rules []struct{ Match, Tag string }) string {
	lower := strings.ToLower(content)
	tags := existingTags
	for _, r := range rules {
		if strings.Contains(lower, strings.ToLower(r.Match)) {
			if tags == "" {
				tags = r.Tag
			} else if !strings.Contains(tags, r.Tag) {
				tags += "," + r.Tag
			}
		}
	}
	return tags
}

func extractTags(content string, fileType string) string {
	var tags []string
	lower := strings.ToLower(content)

	keywords := map[string]string{
		"security":  "보안",
		"auth":      "인증",
		"api":       "api",
		"database":  "데이터베이스",
		"deploy":    "배포",
		"bug":       "버그",
		"decision":  "결정",
		"CEO":       "ceo",
	}

	for eng, tag := range keywords {
		if strings.Contains(lower, strings.ToLower(eng)) || strings.Contains(lower, tag) {
			tags = append(tags, strings.ToLower(eng))
		}
	}

	return strings.Join(tags, ",")
}
