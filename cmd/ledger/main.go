package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cavss/ledger/internal/api"
	"github.com/cavss/ledger/internal/check"
	"github.com/cavss/ledger/internal/config"
	"github.com/cavss/ledger/internal/docpipe"
	"github.com/cavss/ledger/internal/embed"
	"github.com/cavss/ledger/internal/hook"
	"github.com/cavss/ledger/internal/search"
	"github.com/cavss/ledger/internal/snapshot"
	"github.com/cavss/ledger/internal/store"
	"github.com/cavss/ledger/internal/ui"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "init":
		runInit()
	case "index":
		runIndex()
	case "embed":
		runEmbed()
	case "hook":
		runHook()
	case "snapshot":
		runSnapshot()
	case "diff":
		runDiff()
	case "restore":
		runRestore()
	case "check":
		runCheck()
	case "serve":
		runServe()
	case "ui":
		runUI()
	case "search":
		runSearch()
	case "status":
		runStatus()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func getBaseDir() string {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return dir
}

func runIndex() {
	baseDir := getBaseDir()

	// --quiet 플래그
	quiet := false
	for _, arg := range os.Args[2:] {
		if arg == "--quiet" || arg == "-q" {
			quiet = true
		}
	}

	cfg, err := config.Load(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	files, err := cfg.ResolveFiles(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve files error: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		if !quiet {
			fmt.Println("No files found to index.")
		}
		return
	}

	s, err := store.New(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	// 플러그인 설정을 ParseOptions로 변환
	parseOpts := docpipe.ParseOptions{
		ChunkBy:   cfg.Plugins.ChunkBy,
		Separator: cfg.Plugins.Separator,
	}
	if len(cfg.Plugins.FileTypes) > 0 {
		parseOpts.CustomFileTypes = make(map[string]string)
		for typeName, rule := range cfg.Plugins.FileTypes {
			parseOpts.CustomFileTypes[rule.Pattern] = typeName
		}
	}
	for _, r := range cfg.Plugins.TagRules {
		parseOpts.CustomTagRules = append(parseOpts.CustomTagRules, struct{ Match, Tag string }{r.Match, r.Tag})
	}

	var totalInserted, totalUpdated, totalSkipped int

	for _, f := range files {
		changed, err := s.IsFileChanged(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: %v\n", shortPath(f, baseDir), err)
			continue
		}
		if !changed {
			totalSkipped++
			continue
		}

		chunks, err := docpipe.ParseFileWithOptions(f, parseOpts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  parse error %s: %v\n", shortPath(f, baseDir), err)
			continue
		}

		// 기존 청크 제거 후 새로 삽입 (파일 단위 재인덱싱)
		s.RemoveChunksForFile(f)

		inserted, updated, skipped, err := s.UpsertChunks(chunks)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  store error %s: %v\n", shortPath(f, baseDir), err)
			continue
		}

		fileType := ""
		if len(chunks) > 0 {
			fileType = chunks[0].FileType
		}
		s.UpdateMetadata(f, fileType, len(chunks))

		totalInserted += inserted
		totalUpdated += updated
		totalSkipped += skipped

		if !quiet {
			fmt.Printf("  ✓ %s (%d chunks)\n", shortPath(f, baseDir), len(chunks))
		}
	}

	if !quiet {
		fmt.Printf("\nIndexed: %d files, +%d new, ~%d updated, =%d unchanged\n",
			len(files), totalInserted, totalUpdated, totalSkipped)
	}
}

func runHook() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: ledger hook <install|uninstall|status>\n")
		os.Exit(1)
	}

	baseDir := getBaseDir()
	subcmd := os.Args[2]

	switch subcmd {
	case "install":
		if err := hook.Install(baseDir); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ post-commit hook installed")
		fmt.Println("  Documents will be re-indexed automatically on each commit.")

	case "uninstall":
		if err := hook.Uninstall(baseDir); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ post-commit hook removed")

	case "status":
		installed, path := hook.Status(baseDir)
		if installed {
			fmt.Printf("✓ hook installed at %s\n", path)
		} else {
			fmt.Println("✗ hook not installed")
			fmt.Println("  Run 'ledger hook install' to enable auto-indexing on commit.")
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown hook command: %s\n", subcmd)
		fmt.Fprintf(os.Stderr, "usage: ledger hook <install|uninstall|status>\n")
		os.Exit(1)
	}
}

func runSnapshot() {
	baseDir := getBaseDir()
	s, err := store.New(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	// ledger snapshot list
	if len(os.Args) >= 3 && os.Args[2] == "list" {
		snaps, err := snapshot.List(s.DB())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(snaps) == 0 {
			fmt.Println("No snapshots yet. Run 'ledger snapshot' to create one.")
			return
		}
		fmt.Printf("%-4s  %-10s  %-20s  %5s  %6s  %s\n", "ID", "COMMIT", "LABEL", "FILES", "CHUNKS", "DATE")
		for _, snap := range snaps {
			fmt.Printf("%-4d  %-10s  %-20s  %5d  %6d  %s\n",
				snap.ID, snap.CommitHash[:10], snap.Label,
				snap.TotalFiles, snap.TotalChunks,
				snap.CreatedAt.Format("2006-01-02 15:04"))
		}
		return
	}

	// ledger snapshot [--label "..."]
	label := ""
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "--label" && i+1 < len(os.Args) {
			label = os.Args[i+1]
			i++
		}
	}

	snap, err := snapshot.Create(s.DB(), baseDir, label)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Snapshot #%d created\n", snap.ID)
	fmt.Printf("  Commit: %s\n", snap.CommitHash[:10])
	fmt.Printf("  Files:  %d\n", snap.TotalFiles)
	fmt.Printf("  Chunks: %d\n", snap.TotalChunks)
	if label != "" {
		fmt.Printf("  Label:  %s\n", label)
	}
}

func runDiff() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "usage: ledger diff <ref-a> <ref-b>\n")
		fmt.Fprintf(os.Stderr, "  refs can be: commit hash, HEAD, HEAD~N\n")
		os.Exit(1)
	}

	refA := os.Args[2]
	refB := os.Args[3]
	baseDir := getBaseDir()

	s, err := store.New(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	// git ref → commit hash → snapshot 찾기
	commitA, err := snapshot.ResolveRef(baseDir, refA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	commitB, err := snapshot.ResolveRef(baseDir, refB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	snapA, err := snapshot.FindByCommit(s.DB(), commitA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: no snapshot for %s (%s)\n", refA, commitA[:10])
		fmt.Fprintf(os.Stderr, "Hint: run 'ledger snapshot' after each commit\n")
		os.Exit(1)
	}
	snapB, err := snapshot.FindByCommit(s.DB(), commitB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: no snapshot for %s (%s)\n", refB, commitB[:10])
		fmt.Fprintf(os.Stderr, "Hint: run 'ledger snapshot' after each commit\n")
		os.Exit(1)
	}

	diff, err := snapshot.Diff(s.DB(), snapA.ID, snapB.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Diff: %s..%s\n\n", diff.FromCommit[:10], diff.ToCommit[:10])

	if len(diff.Added) > 0 {
		fmt.Printf("+ Added (%d chunks):\n", len(diff.Added))
		for _, d := range diff.Added {
			heading := d.Heading
			if heading == "" {
				heading = "(no heading)"
			}
			fmt.Printf("  + %s — %s\n", shortPath(d.File, baseDir), heading)
		}
		fmt.Println()
	}

	if len(diff.Removed) > 0 {
		fmt.Printf("- Removed (%d chunks):\n", len(diff.Removed))
		for _, d := range diff.Removed {
			heading := d.Heading
			if heading == "" {
				heading = "(no heading)"
			}
			fmt.Printf("  - %s — %s\n", shortPath(d.File, baseDir), heading)
		}
		fmt.Println()
	}

	if len(diff.Added) == 0 && len(diff.Removed) == 0 {
		fmt.Println("No changes between snapshots.")
	} else {
		fmt.Printf("Summary: +%d chunks, -%d chunks\n",
			diff.Summary.ChunksAdded, diff.Summary.ChunksRemoved)
	}
}

func runRestore() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: ledger restore <ref>\n")
		fmt.Fprintf(os.Stderr, "  ref: commit hash, HEAD~N, or snapshot ID\n")
		os.Exit(1)
	}

	ref := os.Args[2]
	baseDir := getBaseDir()

	s, err := store.New(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	// commit hash로 시도
	commitHash, err := snapshot.ResolveRef(baseDir, ref)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	snap, err := snapshot.FindByCommit(s.DB(), commitHash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: no snapshot for %s\n", ref)
		os.Exit(1)
	}

	if err := snapshot.Restore(s.DB(), snap.ID); err != nil {
		fmt.Fprintf(os.Stderr, "restore error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Restored index to snapshot #%d (%s)\n", snap.ID, snap.CommitHash[:10])
	fmt.Printf("  Files:  %d\n", snap.TotalFiles)
	fmt.Printf("  Chunks: %d\n", snap.TotalChunks)
}

func runServe() {
	baseDir := getBaseDir()
	s, err := store.New(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store error: %v\n", err)
		os.Exit(1)
	}

	port := api.ParsePort(os.Args[2:], 7890)
	srv := api.NewServer(s, baseDir, port)
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func runUI() {
	baseDir := getBaseDir()
	s, err := store.New(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store error: %v\n", err)
		os.Exit(1)
	}

	port := api.ParsePort(os.Args[2:], 7890)
	if err := ui.StartUI(s, baseDir, port); err != nil {
		fmt.Fprintf(os.Stderr, "ui error: %v\n", err)
		os.Exit(1)
	}
}

func runCheck() {
	baseDir := getBaseDir()
	s, err := store.New(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	showFix := false
	threshold := 0.85
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--fix":
			showFix = true
		case "--threshold":
			if i+1 < len(os.Args) {
				fmt.Sscanf(os.Args[i+1], "%f", &threshold)
				i++
			}
		}
	}

	fmt.Println("Scanning for contradictions...")
	result, err := check.Run(s.DB(), s, threshold)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check error: %v\n", err)
		os.Exit(1)
	}

	total := len(result.Contradictions) + len(result.Superseded)
	if total == 0 {
		fmt.Println("✓ No contradictions found.")
		return
	}

	if len(result.Contradictions) > 0 {
		fmt.Printf("\n⚠ Contradictions (%d):\n\n", len(result.Contradictions))
		for i, c := range result.Contradictions {
			fmt.Printf("[%d] similarity: %.2f — %s\n", i+1, c.Similarity, c.Reason)
			fmt.Printf("    A: %s — %s\n", shortPath(c.ChunkA.File, baseDir), c.ChunkA.Heading)
			fmt.Printf("    B: %s — %s\n", shortPath(c.ChunkB.File, baseDir), c.ChunkB.Heading)
			if showFix {
				fmt.Printf("    → %s\n", "Review both and consolidate into the authoritative source.")
			}
			fmt.Println()
		}
	}

	if len(result.Superseded) > 0 {
		fmt.Printf("⚠ Possibly superseded (%d):\n\n", len(result.Superseded))
		for i, sp := range result.Superseded {
			fmt.Printf("[%d] similarity: %.2f\n", i+1, sp.Similarity)
			fmt.Printf("    Older: %s — %s (date: %s)\n", shortPath(sp.Older.File, baseDir), sp.Older.Heading, sp.Older.Date)
			fmt.Printf("    Newer: %s — %s (date: %s)\n", shortPath(sp.Newer.File, baseDir), sp.Newer.Heading, sp.Newer.Date)
			if showFix {
				fmt.Printf("    → %s\n", "Mark older document as superseded or archive it.")
			}
			fmt.Println()
		}
	}

	fmt.Printf("Total: %d issues found\n", total)
}

func runEmbed() {
	baseDir := getBaseDir()

	cfg, err := config.Load(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	s, err := store.New(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	model := cfg.Embedding.Model
	endpoint := cfg.Embedding.Endpoint

	// CLI에서 --model, --endpoint 오버라이드
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--model":
			if i+1 < len(os.Args) {
				model = os.Args[i+1]
				i++
			}
		case "--endpoint":
			if i+1 < len(os.Args) {
				endpoint = os.Args[i+1]
				i++
			}
		}
	}

	client := embed.NewClient(endpoint, model)

	fmt.Printf("Connecting to Ollama at %s ...\n", endpoint)
	if err := client.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Hint: run 'ollama serve' and 'ollama pull %s'\n", model)
		os.Exit(1)
	}

	// 임베딩 없는 청크 찾기
	ids, err := s.GetChunksWithoutVectors()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(ids) == 0 {
		fmt.Println("All chunks already have embeddings.")
		return
	}

	fmt.Printf("Generating embeddings for %d chunks (model: %s)...\n", len(ids), model)

	var success, failed int
	for i, id := range ids {
		content, err := s.GetChunkContent(id)
		if err != nil {
			failed++
			continue
		}

		vec, err := client.Embed(content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ chunk %d: %v\n", id, err)
			failed++
			// 연속 3회 실패 시 중단
			if failed >= 3 && success == 0 {
				fmt.Fprintf(os.Stderr, "\nAborted: model may not be available. Run 'ollama pull %s'\n", model)
				os.Exit(1)
			}
			continue
		}

		if err := s.StoreVector(id, vec, model); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ store chunk %d: %v\n", id, err)
			failed++
			continue
		}

		success++
		if (i+1)%10 == 0 || i+1 == len(ids) {
			fmt.Printf("  [%d/%d] embedded\n", i+1, len(ids))
		}
	}

	fmt.Printf("\nDone: %d embedded, %d failed\n", success, failed)
}

func runSearch() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: ledger search <query> [--type TYPE] [--tag TAG] [--top N]\n")
		os.Exit(1)
	}

	query := os.Args[2]
	topK := 5
	mode := "keyword" // keyword | semantic | hybrid
	role := ""
	var filters []search.Filter

	// 간단한 플래그 파싱
	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--type":
			if i+1 < len(os.Args) {
				filters = append(filters, search.FileTypeFilter{FileType: os.Args[i+1]})
				i++
			}
		case "--tag":
			if i+1 < len(os.Args) {
				filters = append(filters, search.TagFilter{Tag: os.Args[i+1]})
				i++
			}
		case "--after":
			if i+1 < len(os.Args) {
				filters = append(filters, search.DateAfterFilter{Date: os.Args[i+1]})
				i++
			}
		case "--before":
			if i+1 < len(os.Args) {
				filters = append(filters, search.DateBeforeFilter{Date: os.Args[i+1]})
				i++
			}
		case "--top":
			if i+1 < len(os.Args) {
				fmt.Sscanf(os.Args[i+1], "%d", &topK)
				i++
			}
		case "--mode":
			if i+1 < len(os.Args) {
				mode = os.Args[i+1]
				i++
			}
		case "--role":
			if i+1 < len(os.Args) {
				role = os.Args[i+1]
				i++
			}
		}
	}

	baseDir := getBaseDir()

	cfg, err := config.Load(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	s, err := store.New(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	searcher := search.NewWithStore(s)

	var results []search.Result

	switch mode {
	case "semantic", "hybrid":
		client := embed.NewClient(cfg.Embedding.Endpoint, cfg.Embedding.Model)
		queryVec, err := client.Embed(query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "embedding error: %v\n", err)
			fmt.Fprintf(os.Stderr, "Hint: run 'ollama serve' and 'ledger embed' first\n")
			os.Exit(1)
		}

		if mode == "semantic" {
			results, err = searcher.SemanticSearch(queryVec, topK, filters...)
		} else {
			results, err = searcher.HybridSearch(query, queryVec, topK, 0.5, filters...)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "search error: %v\n", err)
			os.Exit(1)
		}
	default: // keyword
		results, err = searcher.Search(query, topK, filters...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "search error: %v\n", err)
			os.Exit(1)
		}
	}

	// 역할별 가중치 적용
	if role != "" {
		results = search.ApplyRoleBoost(results, role)
	}

	if len(results) == 0 {
		fmt.Println("검색 결과 없음.")
		return
	}

	for i, r := range results {
		fmt.Printf("[%d] %s:%d (score: %.2f)\n", i+1, shortPath(r.File, baseDir), r.LineStart, r.Score)
		if r.Heading != "" {
			fmt.Printf("    %s\n", r.Heading)
		}
		// 내용 미리보기 (최대 3줄)
		preview := previewContent(r.Content, 3)
		for _, line := range preview {
			fmt.Printf("    %s\n", line)
		}
		fmt.Println()
	}
}

func runStatus() {
	baseDir := getBaseDir()
	s, err := store.New(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store error: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	chunks, files, _ := s.GetStats()
	vecCount, vecModel := s.GetVectorStats()
	fmt.Printf("Ledger Status\n")
	fmt.Printf("  DB: %s\n", filepath.Join(baseDir, store.DBDir, store.DBFile))
	fmt.Printf("  Files indexed: %d\n", files)
	fmt.Printf("  Total chunks:  %d\n", chunks)
	fmt.Printf("  Embeddings:    %d", vecCount)
	if vecModel != "" {
		fmt.Printf(" (%s)", vecModel)
	}
	fmt.Println()
}

func runInit() {
	baseDir := getBaseDir()
	configPath := filepath.Join(baseDir, config.DefaultConfigFile)

	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf(".ledger.toml already exists at %s\n", configPath)
		return
	}

	defaultToml := `# Ledger Configuration

[index]
paths = ["**/*.md"]
exclude = ["node_modules/**", ".ledger/**", ".git/**"]
watch = false

[search]
default_top_k = 5
min_score = 0.3

[project]
name = ""
`
	if err := os.WriteFile(configPath, []byte(defaultToml), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error creating config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created %s\n", configPath)

	// git repo면 hook도 자동 설치
	if err := hook.Install(baseDir); err == nil {
		fmt.Println("✓ post-commit hook installed (auto-indexing on commit)")
	}

	fmt.Println("Run 'ledger index' to start indexing.")
}

func printUsage() {
	fmt.Println(`Ledger — Document Memory System

Usage:
  ledger init                      Create .ledger.toml config
  ledger index [--quiet]           Index documents
  ledger embed [flags]             Generate embeddings (requires Ollama)
  ledger search <query> [flags]    Search indexed documents
  ledger snapshot [--label "..."]  Save index snapshot at current commit
  ledger snapshot list             List all snapshots
  ledger diff <ref-a> <ref-b>     Compare two snapshots (HEAD, HEAD~N, hash)
  ledger restore <ref>            Restore index to a snapshot
  ledger check [--fix] [--threshold N]  Scan for contradictions
  ledger serve [--port N]          Start JSON API server (default: 7890)
  ledger ui [--port N]             Open web UI (default: 7890)
  ledger hook <install|uninstall|status>  Manage git post-commit hook
  ledger status                    Show index status

Search flags:
  --type <type>      Filter by file type (changelog, issues, rules, etc.)
  --tag <tag>        Filter by tag
  --after <date>     Filter by date (YYYY-MM-DD)
  --before <date>    Filter by date (YYYY-MM-DD)
  --top <N>          Number of results (default: 5)
  --mode <mode>      Search mode: keyword (default), semantic, hybrid
  --role <role>      Role-based boost (28 roles supported):
                     director, pm, system_architect, data_architect, cloud_architect,
                     api_developer, business_logic_developer, database_developer,
                     web_developer, ios_developer, android_developer, ui_ux_designer,
                     app_security, db_security, infra_security, security_researcher,
                     functional_tester, performance_tester, cicd_engineer, infra_engineer,
                     tech_writer, data_analyst, tech_researcher, market_researcher,
                     accountant, growth_marketer, content_marketer,
                     developer, security, qa (fallback)

Embed flags:
  --model <name>     Ollama model (default: nomic-embed-text)
  --endpoint <url>   Ollama endpoint (default: http://localhost:11434)`)
}

func shortPath(fullPath, baseDir string) string {
	rel, err := filepath.Rel(baseDir, fullPath)
	if err != nil {
		return fullPath
	}
	return rel
}

func previewContent(content string, maxLines int) []string {
	lines := strings.Split(content, "\n")
	// 헤딩 라인 스킵
	start := 0
	for start < len(lines) && (strings.TrimSpace(lines[start]) == "" || strings.HasPrefix(lines[start], "#")) {
		start++
	}
	var preview []string
	for i := start; i < len(lines) && len(preview) < maxLines; i++ {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			if len(line) > 100 {
				line = line[:100] + "..."
			}
			preview = append(preview, line)
		}
	}
	return preview
}
