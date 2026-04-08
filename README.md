<p align="center">
  <img src="banner.png" alt="Consigliere" width="100%">
</p>

# Consigliere

> **Document Memory System** — LLM 에이전트를 위한 외장 메모리

[![Go](https://img.shields.io/badge/Go-1.18+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![SQLite](https://img.shields.io/badge/SQLite-FTS5-003B57?logo=sqlite&logoColor=white)](https://www.sqlite.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

### CLI Aliases

```
consigliere <command>       # 정식 명칭
--vincenzo                  # 별칭
--consigliere               # 별칭
```

> *Part of the [CLI Company](https://github.com/users/Lee-JuYeon/projects/21) ecosystem*

프로젝트의 문서(.md)를 자동 인덱싱하고, 필요한 컨텍스트만 검색해서 LLM에 주입한다.
전체 파일을 로드하는 대신 **상위 K개 청크만 전달**해서 토큰을 절약하고, 할루시네이션 없이 원본 텍스트만 반환한다.

---

## 왜 Consigliere인가?

| 기존 방식 | 문제 | Consigliere |
|-----------|------|--------|
| `grep "보안"` | 키워드 안 맞으면 못 찾음 | 시맨틱 검색으로 의미 기반 탐색 |
| 전체 파일 로드 | 토큰 폭발 (50+개 MD) | top-K 검색으로 필요한 것만 |
| 수동 문서 관리 | 파일 늘수록 관리 불가 | 자동 인덱싱 + 태깅 |
| 모순 방치 | 옛 규칙과 새 결정 공존 | 자동 모순 감지 + 해결 제안 |
| 세션 끊기면 리셋 | 컨텍스트 유실 | 버전 스냅샷 + 복원 |

### 경쟁 비교

| 기능 | SuperMemory | NotebookLM | **Consigliere** |
|------|:-----------:|:----------:|:----------:|
| 시맨틱 검색 | O | O | **O** |
| 할루시네이션 제로 | △ | O | **O** |
| 버전 추적 + diff | X | X | **O** |
| CLI 호출 | X | X | **O** |
| 역할별 검색 우선순위 | X | X | **O** |
| 모순 감지 | △ | X | **O** |
| Self-hosted / 오프라인 | △ | X | **O** |
| 완전 무료 | △ | O | **O** |

---

## 특징

- **할루시네이션 제로** — 원본 텍스트만 반환. LLM이 생성/추론하지 않음
- **~13MB 단일 바이너리** — 외부 의존성 0, 완전 오프라인 동작
- **3가지 검색 모드** — BM25 키워드 / 시맨틱(벡터) / 하이브리드
- **역할별 우선순위** — developer, director, security 등 역할에 따른 결과 부스트
- **모순 감지** — 문서 간 충돌 자동 스캔 + 해결 제안
- **버전 스냅샷** — 커밋별 인덱스 저장, diff, 시점 복원
- **Git Hook 자동화** — 커밋마다 자동 재인덱싱
- **웹 UI** — 브라우저 기반 검색/브라우즈/체크 대시보드
- **JSON API + Node.js SDK** — 외부 시스템 연동
- **플러그인** — 커스텀 파일 타입, 태그 룰, 청킹 전략

---

## Quick Start

### 설치

```bash
# 소스에서 빌드
git clone https://github.com/Lee-JuYeon/Consigliere.git
cd Consigliere
make build

# 또는 직접 빌드
CGO_ENABLED=1 go build -tags "fts5" -o bin/consigliere ./cmd/consigliere/
```

> **요구사항**: Go 1.18+, CGO 지원 (SQLite FTS5)

### 기본 사용법

```bash
# 1. 프로젝트에서 초기화
cd your-project/
consigliere init              # .consigliere.toml 생성 + git hook 설치

# 2. 문서 인덱싱
consigliere index             # docs/**/*.md 자동 탐색 + 인덱싱

# 3. 검색
consigliere search "보안"
consigliere search "에이전트" --type changelog
consigliere search "버그" --tag security --top 10

# 4. 상태 확인
consigliere status
```

### 시맨틱 검색 (선택)

```bash
# Ollama 설치 후
ollama pull nomic-embed-text

# 임베딩 생성
consigliere embed

# 의미 기반 검색
consigliere search "아키텍처 설계 방향" --mode semantic

# 키워드 + 시맨틱 결합
consigliere search "아키텍처 설계 방향" --mode hybrid
```

### 역할별 검색

```bash
# 개발자 → changelog, issues 우선
consigliere search "프로젝트" --role developer

# 디렉터 → dashboard, workflow 우선
consigliere search "프로젝트" --role director

# 보안팀 → rules, issues 우선
consigliere search "인증" --role security
```

---

## 전체 명령어

```
consigliere init                              설정 파일 생성 + git hook 자동 설치
consigliere index [--quiet]                   문서 인덱싱 (증분 업데이트)
consigliere embed [--model M] [--endpoint U]  벡터 임베딩 생성 (Ollama 연동)
consigliere search <query> [flags]            검색
consigliere snapshot [--label "..."]          현재 인덱스 스냅샷 저장
consigliere snapshot list                     스냅샷 목록 조회
consigliere diff <ref-a> <ref-b>              두 스냅샷 비교 (HEAD, HEAD~N, hash)
consigliere restore <ref>                     스냅샷 시점으로 인덱스 복원
consigliere check [--fix] [--threshold N]     문서 간 모순 감지
consigliere serve [--port N]                  JSON API 서버 시작 (기본: 7890)
consigliere ui [--port N]                     웹 UI 시작 (기본: 7890)
consigliere hook install|uninstall|status     git post-commit hook 관리
consigliere status                            인덱스 상태 확인
```

### Search Flags

| Flag | 설명 | 예시 |
|------|------|------|
| `--type <type>` | 파일 유형 필터 | `--type changelog` |
| `--tag <tag>` | 태그 필터 | `--tag security` |
| `--after <date>` | 날짜 이후 | `--after 2026-03-01` |
| `--before <date>` | 날짜 이전 | `--before 2026-04-01` |
| `--top <N>` | 결과 수 (기본: 5) | `--top 10` |
| `--mode <mode>` | 검색 모드 | `--mode hybrid` |
| `--role <role>` | 역할별 부스트 | `--role developer` |

**검색 모드:**
- `keyword` (기본) — SQLite FTS5 BM25 랭킹
- `semantic` — Ollama 벡터 임베딩 + 코사인 유사도
- `hybrid` — BM25 + 코사인 정규화 결합 (alpha=0.5)

**역할:**
`developer` · `ios_developer` · `director` · `security` · `qa`

---

## 설정

`consigliere init`으로 생성되는 `.consigliere.toml`:

```toml
[index]
paths = ["docs/**/*.md", "CLAUDE.md"]
exclude = ["node_modules/**", ".consigliere/**", ".git/**"]

[search]
default_top_k = 5
min_score = 0.3

[project]
name = "my-project"

# 시맨틱 검색 설정 (선택)
[embedding]
enabled = false
model = "nomic-embed-text"
endpoint = "http://localhost:11434"
dimensions = 768
```

### 플러그인 설정

```toml
# 커스텀 파일 유형 인식
[plugins.file_types.spec]
pattern = "SPEC"          # 파일명에 "SPEC" 포함 → spec 타입

[plugins.file_types.meeting]
pattern = "MEETING"

# 커스텀 태그 자동 부여
[[plugins.tag_rules]]
match = "TODO"            # 내용에 "TODO" 포함 시
tag = "todo"              # "todo" 태그 자동 부여

[[plugins.tag_rules]]
match = "FIXME"
tag = "fixme"

# 청킹 전략 변경
[plugins]
chunk_by = "heading"      # heading (기본) | paragraph | separator
# separator = "---"       # chunk_by = "separator" 일 때
```

---

## 아키텍처

```
┌───────────────────────────────────────────────────────┐
│                       Consigliere                           │
│                                                       │
│  ┌─────────────────────────────────────┐              │
│  │           DocumentPipe               │              │
│  │  MD 파서 → 청킹 → 자동 태깅          │              │
│  │  전략: heading / paragraph / separator│              │
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
│  │BM25  │  │diff    │  │  모순     │                   │
│  │Sem.  │  │restore │  │  감지     │                   │
│  │Hybrid│  └────────┘  └──────────┘                   │
│  │+Role │                                             │
│  └──┬───┘                                             │
│     ▼                                                  │
│  ┌─────────────────────────────────────┐              │
│  │     API Server / Web UI / SDK        │              │
│  │  REST JSON · 대시보드 · Node.js       │              │
│  └─────────────────────────────────────┘              │
│                                                       │
│  ┌──────────────────────┐                             │
│  │ Git Hook (post-commit)│                             │
│  │ 커밋 → 자동 재인덱싱   │                             │
│  └──────────────────────┘                             │
└───────────────────────────────────────────────────────┘
```

### 검색 흐름

```
쿼리 입력
  │
  ├─ keyword ──→ FTS5 MATCH → BM25 rank → 필터 → 결과
  │
  ├─ semantic ─→ Ollama embed(쿼리) → 전체 벡터 코사인 유사도 → top-K
  │
  └─ hybrid ──→ BM25 후보 + 시맨틱 후보
                → 점수 정규화 → (1-α)×BM25 + α×cosine
                → 통합 정렬 → top-K
                     │
                     └─ --role 지정 시: file_type별 가중치 곱 → 재정렬
```

---

## API

`consigliere serve` 또는 `consigliere ui`로 서버 시작:

```bash
consigliere serve --port 7890
```

### 엔드포인트

| Method | Path | 설명 |
|--------|------|------|
| GET | `/api/search?q=보안&top=5&type=changelog&role=developer` | 검색 |
| GET | `/api/status` | 인덱스 상태 |
| GET | `/api/chunks?file=README&type=changelog&limit=50` | 청크 조회 |
| GET | `/api/check?threshold=0.9` | 모순 감지 |
| GET | `/api/health` | 헬스체크 |

### 응답 예시

```json
{
  "query": "보안",
  "results": [
    {
      "id": 42,
      "file": "docs/rules.md",
      "heading": "## 보안 정책",
      "content": "모든 API 키는 환경변수로 관리...",
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

// 검색
const { results } = await consigliere.search('보안', {
  top: 5,
  role: 'developer'
});

// 상태
const status = await consigliere.status();
// { files_indexed: 55, total_chunks: 682, embeddings: 682 }

// 모순 감지
const { contradictions, superseded } = await consigliere.check({
  threshold: 0.9
});
```

TypeScript 타입 정의 (`index.d.ts`) 포함.

---

## 웹 UI

```bash
consigliere ui
# → http://localhost:7890
```

3개 탭:
- **Search** — 쿼리 검색 (모드/역할/필터 선택)
- **Browse** — 전체 청크 목록 조회
- **Check** — 모순 감지 대시보드

상단 상태바에 파일 수, 청크 수, 임베딩 수, 모델명 실시간 표시.

---

## 버전 관리

```bash
# 현재 인덱스 상태를 스냅샷으로 저장
consigliere snapshot --label "v1.0 릴리즈"

# 스냅샷 목록
consigliere snapshot list
# ID    COMMIT      LABEL                 FILES  CHUNKS  DATE
# 2     a6d300abc9  v1.0 릴리즈               5      72  2026-03-31 14:30
# 1     251d3c48e8  초기                      3      45  2026-03-31 10:00

# 두 시점 비교
consigliere diff HEAD~1 HEAD
# + Added (3 chunks):
#   + docs/rules.md — ## 새 보안 정책
#   + docs/CHANGELOG.md — ### 2026-03-31
# - Removed (1 chunk):
#   - docs/rules.md — ## 구 보안 정책
# Summary: +3 chunks, -1 chunks

# 이전 시점으로 복원
consigliere restore HEAD~1
```

---

## 모순 감지

```bash
# 기본 스캔 (threshold: 0.85)
consigliere check

# 높은 정확도
consigliere check --threshold 0.95

# 해결 제안 포함
consigliere check --fix
# ⚠ Contradictions (2):
# [1] similarity: 0.97
#     A: docs/rules.md — ## PM 역할
#     B: docs/CHANGELOG.md — ### Director 변경
#     → Review both and consolidate into the authoritative source.
```

---

## Git Hook

```bash
# 설치 — 커밋마다 자동으로 consigliere index 실행
consigliere hook install
# ✓ post-commit hook installed

# 상태 확인
consigliere hook status
# ✓ hook installed at .git/hooks/post-commit

# 제거
consigliere hook uninstall
```

기존 post-commit hook이 있으면 마커 기반으로 공존. 제거 시 Consigliere 블록만 삭제.

---

## LLM 에이전트 연동 예시

### CLI에서 컨텍스트 주입

```bash
# 에이전트 스폰 전, 역할에 맞는 문서 검색
CONTEXT=$(bin/consigliere search "현재 이슈" --role developer --top 3)

# system prompt에 주입
echo "참고 문서:\n$CONTEXT" | claude --system-prompt -
```

### Node.js 서버에서

```javascript
const { ConsigliereClient } = require('@cavss/consigliere');
const consigliere = new ConsigliereClient();

async function getAgentContext(role, task) {
  const { results } = await consigliere.search(task, { role, top: 5 });
  return results.map(r =>
    `[${r.file}:${r.line_start}] ${r.heading}\n${r.content}`
  ).join('\n---\n');
}

// 에이전트 스폰 시
const context = await getAgentContext('ios_developer', '로그인 화면 구현');
spawnAgent({ systemPrompt: basePrompt + '\n\n' + context });
```

---

## 프로젝트 구조

```
consigliere/
├── cmd/consigliere/main.go            CLI 엔트리포인트 (15개 서브커맨드)
├── internal/
│   ├── api/server.go             JSON REST API (5 엔드포인트)
│   ├── check/check.go            모순 감지 엔진
│   ├── config/config.go          TOML 설정 + 플러그인
│   ├── docpipe/parser.go         MD 파싱 + 3가지 청킹 전략
│   ├── embed/ollama.go           Ollama 임베딩 클라이언트
│   ├── hook/hook.go              Git hook 관리
│   ├── search/search.go          BM25 + 시맨틱 + 하이브리드 + 역할 부스트
│   ├── snapshot/snapshot.go      버전 스냅샷 + diff + restore
│   ├── store/
│   │   ├── db.go                 SQLite 스키마 (5 테이블)
│   │   ├── chunks.go             청크 CRUD + 증분 업데이트
│   │   └── vectors.go            벡터 BLOB + 코사인 유사도
│   └── ui/
│       ├── ui.go                 웹 UI 서버 (Go embed)
│       └── index.html            SPA 대시보드
├── sdk/node/                     Node.js SDK
│   ├── index.js                  ConsigliereClient 클래스
│   ├── index.d.ts                TypeScript 타입
│   └── package.json
├── docs/
│   ├── ARCHITECTURE.md           아키텍처 상세
│   ├── CHANGELOG.md              전체 변경 이력
│   └── ROADMAP.md                로드맵 (Phase 1~9 완료)
├── .consigliere.toml                  설정 파일
├── .gitignore
├── Makefile                      빌드 자동화
├── go.mod
└── go.sum
```

---

## 성능

| 항목 | 측정값 |
|------|--------|
| 바이너리 크기 | ~13MB |
| 인덱싱 (55파일, 682청크) | < 2초 |
| BM25 검색 | ~10ms |
| 시맨틱 검색 (700벡터) | ~5ms + Ollama RTT |
| 임베딩 생성 (72청크) | ~30초 |
| 피크 메모리 | ~12MB |
| DB 크기 (72청크 + 벡터) | ~500KB |

## 기술 스택

| 구성 | 선택 | 이유 |
|------|------|------|
| 언어 | Go 1.18+ | 단일 바이너리, 크로스 컴파일 |
| DB | SQLite + FTS5 | 서버 불필요, WAL 모드 |
| 임베딩 | Ollama (nomic-embed-text) | 로컬, 무료, 768차원 |
| 벡터 검색 | Go in-memory cosine | 의존성 0 |
| 웹 UI | Go embed + 순수 HTML/JS | 프레임워크 0 |
| SDK | Node.js fetch | 의존성 0 |

---

## 로드맵

| Phase | 기능 | 상태 |
|-------|------|------|
| 1 | DocumentPipe + BM25 + CLI | **완료** |
| 2 | 벡터 임베딩 + 시맨틱 검색 | **완료** |
| 3 | VCS hook 자동 인덱싱 | **완료** |
| 4 | 버전 스냅샷 + diff | **완료** |
| 5 | 모순 감지 | **완료** |
| 6 | 역할별 검색 우선순위 | **완료** |
| 7 | JSON API + Node.js SDK | **완료** |
| 8 | 플러그인 시스템 | **완료** |
| 9 | 웹 UI | **완료** |

---

## 관련 프로젝트

- [**CLI Company**](https://github.com/Lee-JuYeon/CLI_Company) — 16개 AI 에이전트의 컨텍스트 메모리로 사용
- **Beethovain** — 코딩 블렌딩 모델의 외장 메모리
- **Soldato** — 블렌딩 모델이 Consigliere를 통해 코드베이스 이해

---

## License

MIT
