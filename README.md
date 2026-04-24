<p align="center">
  <img src="banner.png" alt="Consigliere" width="100%">
</p>

# Consigliere

> **Document Memory System** — External memory for LLM agents

[![Go](https://img.shields.io/badge/Go-1.18+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![SQLite](https://img.shields.io/badge/SQLite-FTS5-003B57?logo=sqlite&logoColor=white)](https://www.sqlite.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

### CLI Aliases

```
consigliere <command>       # formal name
--vincenzo                  # alias
--consigliere               # alias
```

> *Part of the [CLI Company](https://github.com/users/Lee-JuYeon/projects/21) ecosystem*

Automatically indexes project documents (.md), retrieves only the necessary context, and injects it into the LLM.
Instead of loading entire files, it passes only the **top-K chunks** to save tokens and returns only original text with zero hallucination.

---

## Why Consigliere?

| Existing approach | Problem | Consigliere |
|-----------|------|--------|
| `grep "security"` | Can't find if keyword doesn't match | Semantic search for meaning-based discovery |
| Load entire files | Token explosion (50+ MDs) | top-K search for only what's needed |
| Manual document management | Unmanageable as files grow | Automatic indexing + tagging |
| Ignore contradictions | Old rules coexist with new decisions | Automatic contradiction detection + resolution suggestions |
| Reset on session end | Context lost | Version snapshots + restore |

### Competitive Comparison

| Feature | SuperMemory | NotebookLM | **Consigliere** |
|------|:-----------:|:----------:|:----------:|
| Semantic search | O | O | **O** |
| Zero hallucination | △ | O | **O** |
| Version tracking + diff | X | X | **O** |
| CLI invocation | X | X | **O** |
| Role-based search priority | X | X | **O** |
| Contradiction detection | △ | X | **O** |
| Self-hosted / offline | △ | X | **O** |
| Completely free | △ | O | **O** |

---

## Features

- **Zero hallucination** — Returns only original text. LLM does not generate or infer
- **~13MB single binary** — Zero external dependencies, fully offline operation
- **3 search modes** — BM25 keyword / semantic (vector) / hybrid
- **Role-based priority** — Result boosting based on roles like developer, director, security
- **Contradiction detection** — Automatic scan for conflicts between documents + resolution suggestions
- **Version snapshots** — Index saved per commit, diff, point-in-time restore
- **Git Hook automation** — Automatic re-indexing on every commit
- **Web UI** — Browser-based search/browse/check dashboard
- **JSON API + Node.js SDK** — Integration with external systems
- **Plugins** — Custom file types, tag rules, chunking strategies

---

## Quick Start

### Installation

```bash
# Build from source
git clone https://github.com/Lee-JuYeon/Consigliere.git
cd Consigliere
make build

# Or build directly
CGO_ENABLED=1 go build -tags "fts5" -o bin/consigliere ./cmd/consigliere/
```

> **Requirements**: Go 1.18+, CGO support (SQLite FTS5)

### Basic Usage

```bash
# 1. Initialize in your project
cd your-project/
consigliere init              # creates .consigliere.toml + installs git hook

# 2. Index documents
consigliere index             # auto-discovers + indexes docs/**/*.md

# 3. Search
consigliere search "security"
consigliere search "agent" --type changelog
consigliere search "bug" --tag security --top 10

# 4. Check status
consigliere status
```

### Semantic Search (optional)

```bash
# After installing Ollama
ollama pull nomic-embed-text

# Generate embeddings
consigliere embed

# Meaning-based search
consigliere search "architecture design direction" --mode semantic

# Keyword + semantic combined
consigliere search "architecture design direction" --mode hybrid
```

### Role-based Search

```bash
# Developer → changelog, issues first
consigliere search "project" --role developer

# Director → dashboard, workflow first
consigliere search "project" --role director

# Security team → rules, issues first
consigliere search "authentication" --role security
```

---

## All Commands

```
consigliere init                              Create config file + auto-install git hook
consigliere index [--quiet]                   Index documents (incremental update)
consigliere embed [--model M] [--endpoint U]  Generate vector embeddings (Ollama integration)
consigliere search <query> [flags]            Search
consigliere snapshot [--label "..."]          Save current index as snapshot
consigliere snapshot list                     List snapshots
consigliere diff <ref-a> <ref-b>              Compare two snapshots (HEAD, HEAD~N, hash)
consigliere restore <ref>                     Restore index to snapshot point
consigliere check [--fix] [--threshold N]     Detect contradictions between documents
consigliere serve [--port N]                  Start JSON API server (default: 7890)
consigliere ui [--port N]                     Start web UI (default: 7890)
consigliere hook install|uninstall|status     Manage git post-commit hook
consigliere status                            Check index status
```

### Search Flags

| Flag | Description | Example |
|------|------|------|
| `--type <type>` | File type filter | `--type changelog` |
| `--tag <tag>` | Tag filter | `--tag security` |
| `--after <date>` | After date | `--after 2026-03-01` |
| `--before <date>` | Before date | `--before 2026-04-01` |
| `--top <N>` | Result count (default: 5) | `--top 10` |
| `--mode <mode>` | Search mode | `--mode hybrid` |
| `--role <role>` | Role-based boost | `--role developer` |

**Search modes:**
- `keyword` (default) — SQLite FTS5 BM25 ranking
- `semantic` — Ollama vector embeddings + cosine similarity
- `hybrid` — BM25 + normalized cosine combined (alpha=0.5)

**Roles:**
`developer` · `ios_developer` · `director` · `security` · `qa`

---

## Configuration

`.consigliere.toml` generated by `consigliere init`:

```toml
[index]
paths = ["docs/**/*.md", "CLAUDE.md"]
exclude = ["node_modules/**", ".consigliere/**", ".git/**"]

[search]
default_top_k = 5
min_score = 0.3

[project]
name = "my-project"

# Semantic search settings (optional)
[embedding]
enabled = false
model = "nomic-embed-text"
endpoint = "http://localhost:11434"
dimensions = 768
```

### Plugin Configuration

```toml
# Custom file type recognition
[plugins.file_types.spec]
pattern = "SPEC"          # filename contains "SPEC" → spec type

[plugins.file_types.meeting]
pattern = "MEETING"

# Custom auto-tagging
[[plugins.tag_rules]]
match = "TODO"            # when content contains "TODO"
tag = "todo"              # auto-assign "todo" tag

[[plugins.tag_rules]]
match = "FIXME"
tag = "fixme"

# Change chunking strategy
[plugins]
chunk_by = "heading"      # heading (default) | paragraph | separator
# separator = "---"       # when chunk_by = "separator"
```

---

## Architecture

```
┌───────────────────────────────────────────────────────┐
│                       Consigliere                           │
│                                                       │
│  ┌─────────────────────────────────────┐              │
│  │           DocumentPipe               │              │
│  │  MD parser → chunking → auto-tagging │              │
│  │  strategy: heading / paragraph / separator│              │
│  └──────────────┬──────────────────────┘              │
│                 ▼                                      │
│  ┌─────────────────────────────────────┐              │
│  │       SQLite (FTS5 + Vectors)        │              │
│  │  chunks · chunks_fts · metadata      │              │
│  │  vectors · snapshots                 │              │
│  └──────────────┬──────────────────────┘              │
│                 │                                      │
│    ┌────────────┼────────────┐                         │
│    ▼            ▼            ▼                         │
│  ┌──────┐  ┌────────┐  ┌──────────┐                  │
│  │Search│  │Snapshot│  │  Checker │                   │
│  │BM25  │  │diff    │  │  Contra- │                   │
│  │Sem.  │  │restore │  │  diction │                   │
│  │Hybrid│  └────────┘  └──────────┘                   │
│  │+Role │                                             │
│  └──┬───┘                                             │
│     ▼                                                  │
│  ┌─────────────────────────────────────┐              │
│  │     API Server / Web UI / SDK        │              │
│  │  REST JSON · Dashboard · Node.js     │              │
│  └─────────────────────────────────────┘              │
│                                                       │
│  ┌──────────────────────┐                             │
│  │ Git Hook (post-commit)│                             │
│  │ commit → auto re-index│                             │
│  └──────────────────────┘                             │
└───────────────────────────────────────────────────────┘
```

### Search Flow

```
Query input
  │
  ├─ keyword ──→ FTS5 MATCH → BM25 rank → filter → results
  │
  ├─ semantic ─→ Ollama embed(query) → full vector cosine similarity → top-K
  │
  └─ hybrid ──→ BM25 candidates + semantic candidates
                → score normalization → (1-α)×BM25 + α×cosine
                → unified sort → top-K
                     │
                     └─ when --role specified: multiply file_type weight → re-rank
```

---

## API

Start the server with `consigliere serve` or `consigliere ui`:

```bash
consigliere serve --port 7890
```

### Endpoints

| Method | Path | Description |
|--------|------|------|
| GET | `/api/search?q=security&top=5&type=changelog&role=developer` | Search |
| GET | `/api/status` | Index status |
| GET | `/api/chunks?file=README&type=changelog&limit=50` | Retrieve chunks |
| GET | `/api/check?threshold=0.9` | Contradiction detection |
| GET | `/api/health` | Health check |

### Response Example

```json
{
  "query": "security",
  "results": [
    {
      "id": 42,
      "file": "docs/rules.md",
      "heading": "## Security Policy",
      "content": "All API keys are managed as environment variables...",
      "file_type": "rules",
      "tags": "security",
      "line_start": 15,
      "line_end": 28,
      "score": 5.44
    }
  ],
  "total": 1
}
```

### Node.js SDK

```bash
cd sdk/node && npm install
```

```javascript
const { ConsigliereClient } = require('@cavss/consigliere');

const consigliere = new ConsigliereClient({ endpoint: 'http://localhost:7890' });

// Search
const { results } = await consigliere.search('security', {
  top: 5,
  role: 'developer'
});

// Status
const status = await consigliere.status();
// { files_indexed: 55, total_chunks: 682, embeddings: 682 }

// Contradiction detection
const { contradictions, superseded } = await consigliere.check({
  threshold: 0.9
});
```

Includes TypeScript type definitions (`index.d.ts`).

---

## Web UI

```bash
consigliere ui
# → http://localhost:7890
```

3 tabs:
- **Search** — Query search (select mode/role/filter)
- **Browse** — View full chunk list
- **Check** — Contradiction detection dashboard

Status bar at the top shows file count, chunk count, embedding count, and model name in real time.

---

## Version Management

```bash
# Save current index state as snapshot
consigliere snapshot --label "v1.0 release"

# List snapshots
consigliere snapshot list
# ID    COMMIT      LABEL                 FILES  CHUNKS  DATE
# 2     a6d300abc9  v1.0 release              5      72  2026-03-31 14:30
# 1     251d3c48e8  initial                   3      45  2026-03-31 10:00

# Compare two points in time
consigliere diff HEAD~1 HEAD
# + Added (3 chunks):
#   + docs/rules.md — ## New Security Policy
#   + docs/CHANGELOG.md — ### 2026-03-31
# - Removed (1 chunk):
#   - docs/rules.md — ## Old Security Policy
# Summary: +3 chunks, -1 chunks

# Restore to a previous point
consigliere restore HEAD~1
```

---

## Contradiction Detection

```bash
# Default scan (threshold: 0.85)
consigliere check

# High precision
consigliere check --threshold 0.95

# Include resolution suggestions
consigliere check --fix
# ⚠ Contradictions (2):
# [1] similarity: 0.97
#     A: docs/rules.md — ## PM Role
#     B: docs/CHANGELOG.md — ### Director Change
#     → Review both and consolidate into the authoritative source.
```

---

## Git Hook

```bash
# Install — automatically runs consigliere index on every commit
consigliere hook install
# ✓ post-commit hook installed

# Check status
consigliere hook status
# ✓ hook installed at .git/hooks/post-commit

# Remove
consigliere hook uninstall
```

If an existing post-commit hook is present, it coexists via marker-based injection. Removal deletes only the Consigliere block.

---

## LLM Agent Integration Examples

### Context injection from CLI

```bash
# Before spawning an agent, search documents matching the role
CONTEXT=$(bin/consigliere search "current issues" --role developer --top 3)

# Inject into system prompt
echo "Reference documents:\n$CONTEXT" | claude --system-prompt -
```

### From a Node.js server

```javascript
const { ConsigliereClient } = require('@cavss/consigliere');
const consigliere = new ConsigliereClient();

async function getAgentContext(role, task) {
  const { results } = await consigliere.search(task, { role, top: 5 });
  return results.map(r =>
    `[${r.file}:${r.line_start}] ${r.heading}\n${r.content}`
  ).join('\n---\n');
}

// When spawning an agent
const context = await getAgentContext('ios_developer', 'implement login screen');
spawnAgent({ systemPrompt: basePrompt + '\n\n' + context });
```

---

## Project Structure

```
consigliere/
├── cmd/consigliere/main.go            CLI entry point (15 subcommands)
├── internal/
│   ├── api/server.go             JSON REST API (5 endpoints)
│   ├── check/check.go            Contradiction detection engine
│   ├── config/config.go          TOML config + plugins
│   ├── docpipe/parser.go         MD parsing + 3 chunking strategies
│   ├── embed/ollama.go           Ollama embedding client
│   ├── hook/hook.go              Git hook management
│   ├── search/search.go          BM25 + semantic + hybrid + role boost
│   ├── snapshot/snapshot.go      Version snapshots + diff + restore
│   ├── store/
│   │   ├── db.go                 SQLite schema (5 tables)
│   │   ├── chunks.go             Chunk CRUD + incremental update
│   │   └── vectors.go            Vector BLOB + cosine similarity
│   └── ui/
│       ├── ui.go                 Web UI server (Go embed)
│       └── index.html            SPA dashboard
├── sdk/node/                     Node.js SDK
│   ├── index.js                  ConsigliereClient class
│   ├── index.d.ts                TypeScript types
│   └── package.json
├── docs/
│   ├── ARCHITECTURE.md           Architecture details
│   ├── CHANGELOG.md              Full change history
│   └── ROADMAP.md                Roadmap (Phase 1~9 complete)
├── .consigliere.toml                  Config file
├── .gitignore
├── Makefile                      Build automation
├── go.mod
└── go.sum
```

---

## Performance

| Item | Measured value |
|------|--------|
| Binary size | ~13MB |
| Indexing (55 files, 682 chunks) | < 2 seconds |
| BM25 search | ~10ms |
| Semantic search (700 vectors) | ~5ms + Ollama RTT |
| Embedding generation (72 chunks) | ~30 seconds |
| Peak memory | ~12MB |
| DB size (72 chunks + vectors) | ~500KB |

## Tech Stack

| Component | Choice | Reason |
|------|------|------|
| Language | Go 1.18+ | Single binary, cross-compilation |
| DB | SQLite + FTS5 | No server required, WAL mode |
| Embeddings | Ollama (nomic-embed-text) | Local, free, 768 dimensions |
| Vector search | Go in-memory cosine | Zero dependencies |
| Web UI | Go embed + pure HTML/JS | Zero frameworks |
| SDK | Node.js fetch | Zero dependencies |

---

## Roadmap

| Phase | Feature | Status |
|-------|------|------|
| 1 | DocumentPipe + BM25 + CLI | **Complete** |
| 2 | Vector embeddings + semantic search | **Complete** |
| 3 | VCS hook auto-indexing | **Complete** |
| 4 | Version snapshots + diff | **Complete** |
| 5 | Contradiction detection | **Complete** |
| 6 | Role-based search priority | **Complete** |
| 7 | JSON API + Node.js SDK | **Complete** |
| 8 | Plugin system | **Complete** |
| 9 | Web UI | **Complete** |

---

## Related Projects

- [**CLI Company**](https://github.com/Lee-JuYeon/CLI_Company) — Used as context memory for 16 AI agents
- **Beethovain** — External memory for coding blending model
- **Soldato** — Blending model understands codebase via Consigliere

---

## License

MIT
