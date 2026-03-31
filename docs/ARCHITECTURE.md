# Ledger Architecture

## 디렉토리 구조

```
ledger/
├── cmd/
│   └── ledger/
│       └── main.go                # CLI 엔트리포인트 (15개 서브커맨드)
├── internal/
│   ├── api/
│   │   └── server.go              # JSON REST API 서버 (5 엔드포인트)
│   ├── check/
│   │   └── check.go               # 모순 감지 (벡터 유사도 + 헤딩 기반)
│   ├── config/
│   │   └── config.go              # .ledger.toml 파싱 + 플러그인 설정
│   ├── docpipe/
│   │   └── parser.go              # MD 파싱 + 청킹 (heading/paragraph/separator)
│   ├── embed/
│   │   └── ollama.go              # Ollama 임베딩 HTTP 클라이언트
│   ├── hook/
│   │   └── hook.go                # Git post-commit hook 관리
│   ├── search/
│   │   └── search.go              # BM25 + 시맨틱 + 하이브리드 + 역할별 부스트
│   ├── snapshot/
│   │   └── snapshot.go            # 버전 스냅샷 생성/비교/복원
│   ├── store/
│   │   ├── db.go                  # SQLite 초기화 + 마이그레이션 (5 테이블)
│   │   ├── chunks.go              # 청크 CRUD + 메타데이터
│   │   └── vectors.go             # 벡터 BLOB 저장 + 코사인 유사도
│   └── ui/
│       ├── ui.go                  # 웹 UI 서버 (Go embed)
│       └── index.html             # 검색/브라우즈/체크 대시보드
├── sdk/
│   └── node/
│       ├── index.js               # Node.js 클라이언트
│       ├── index.d.ts             # TypeScript 타입 정의
│       └── package.json
├── docs/
│   ├── ARCHITECTURE.md            # 이 파일
│   ├── CHANGELOG.md
│   ├── ROADMAP.md
│   └── coding-model-strategy.md   # 코딩 모델 전략 (4개 프로젝트 공유)
├── .ledger.toml
├── .gitignore
├── Makefile
├── README.md
├── go.mod
└── go.sum
```

## 데이터 흐름

```
[MD 파일들]
     │
     ▼
[config] .ledger.toml에서 경로/제외/플러그인 설정 읽기
     │
     ▼
[docpipe] 파일 파싱 → 청킹 → 자동 태깅
     │     청킹 전략: heading (기본) / paragraph / separator
     │     파일 유형 감지: 내장 + 커스텀 룰
     │     자동 태깅: 내장 키워드 + 커스텀 태그 룰
     │     날짜 추출 (YYYY-MM-DD 패턴)
     │
     ▼
[store] SQLite에 저장
     │     chunks — 청크 본문, 메타데이터
     │     chunks_fts — FTS5 전문 검색 인덱스
     │     metadata — 파일 해시 (증분 업데이트)
     │     vectors — 임베딩 BLOB (float32 배열)
     │     snapshots / snapshot_chunks — 버전 스냅샷
     │
     ├──→ [search] 3가지 검색 모드
     │       BM25 키워드 (FTS5 rank)
     │       시맨틱 (코사인 유사도, 전체 벡터 스캔)
     │       하이브리드 (정규화 BM25 + 코사인, alpha=0.5)
     │       + 역할별 가중치 부스트
     │
     ├──→ [check] 모순 감지
     │       벡터 유사도 > threshold & 다른 파일 → 모순
     │       날짜 차이 → superseded 판정
     │       벡터 없으면 헤딩 중복 기반 폴백
     │
     ├──→ [snapshot] 버전 관리
     │       현재 활성 청크 → snapshot_chunks에 복사
     │       diff: content_hash 기반 추가/삭제 비교
     │       restore: 스냅샷의 청크를 다시 활성화
     │
     └──→ [api/ui] 외부 접근
            REST JSON API (5 엔드포인트)
            웹 UI (Go embed, SPA)
            Node.js SDK (fetch 기반)
```

## DB 스키마

```sql
-- 청크 (인덱싱된 문서 조각)
chunks (
  id INTEGER PRIMARY KEY,
  file TEXT, heading TEXT, content TEXT, content_hash TEXT,
  file_type TEXT, date TEXT, tags TEXT, status TEXT,
  line_start INTEGER, line_end INTEGER,
  created_at TIMESTAMP, updated_at TIMESTAMP
)

-- FTS5 전문 검색 인덱스
chunks_fts USING fts5(content, heading, tags)

-- 파일 메타데이터 (증분 업데이트용)
metadata (
  file TEXT PRIMARY KEY,
  file_hash TEXT, file_type TEXT, chunk_count INTEGER,
  last_modified TIMESTAMP, last_indexed TIMESTAMP
)

-- 벡터 임베딩 (BLOB 저장, Go에서 코사인 유사도 계산)
vectors (
  chunk_id INTEGER PRIMARY KEY → chunks(id),
  embedding BLOB,  -- float32[] little-endian 인코딩
  model TEXT,
  created_at TIMESTAMP
)

-- 버전 스냅샷
snapshots (
  id INTEGER PRIMARY KEY,
  commit_hash TEXT, label TEXT,
  total_files INTEGER, total_chunks INTEGER,
  created_at TIMESTAMP
)

-- 스냅샷별 청크 상태
snapshot_chunks (
  snapshot_id INTEGER → snapshots(id),
  chunk_id INTEGER,
  file TEXT, heading TEXT, content_hash TEXT, file_type TEXT,
  PRIMARY KEY (snapshot_id, chunk_id)
)
```

## API 엔드포인트

| Method | Path | 설명 |
|--------|------|------|
| GET | `/api/search?q=&top=&type=&tag=&role=` | 키워드 검색 |
| GET | `/api/status` | 인덱스 상태 |
| GET | `/api/chunks?file=&type=&limit=&offset=` | 청크 조회 |
| GET | `/api/check?threshold=` | 모순 감지 |
| GET | `/api/health` | 헬스체크 |

## 검색 아키텍처

```
[쿼리 입력]
     │
     ├── mode=keyword ──→ FTS5 MATCH → BM25 rank → 결과
     │
     ├── mode=semantic ─→ Ollama embed(쿼리)
     │                    → 전체 벡터 로드 → 코사인 유사도
     │                    → 상위 K개 → 필터 → 결과
     │
     └── mode=hybrid ──→ BM25 후보 (3K개)
                         + 시맨틱 후보 (3K개)
                         → 점수 정규화
                         → (1-α)×BM25 + α×cosine
                         → 통합 정렬 → 상위 K개
                         │
                         └── --role 적용 시:
                             역할별 file_type 가중치 곱
                             → 재정렬
```

## 플러그인 시스템

```toml
# 커스텀 파일 유형
[plugins.file_types.meeting]
pattern = "MEETING"       # 파일명에 "MEETING" 포함 → "meeting" 타입

# 커스텀 태그 룰
[[plugins.tag_rules]]
match = "TODO"            # 내용에 "TODO" 포함 시 → "todo" 태그
tag = "todo"

# 청킹 전략
[plugins]
chunk_by = "heading"      # heading | paragraph | separator
separator = "---"         # chunk_by=separator 일 때 구분자
```

## 기술 스택

| 구성 | 선택 | 이유 |
|------|------|------|
| 언어 | Go 1.18+ | 단일 바이너리, 크로스 컴파일, embed 지원 |
| DB | SQLite + FTS5 | 로컬, 서버 불필요, WAL 모드 |
| 임베딩 | Ollama (nomic-embed-text) | 로컬 실행, 무료, 768차원 |
| 벡터 검색 | Go in-memory cosine | 외부 의존성 없음, ~1000청크까지 충분 |
| TOML | BurntSushi/toml | Go 표준 TOML 파서 |
| SQLite 드라이버 | mattn/go-sqlite3 | CGO, FTS5 지원 |
| 웹 UI | Go embed + 순수 HTML/JS | 외부 프레임워크 없음, 바이너리 내장 |

## 성능

| 항목 | 측정값 |
|------|--------|
| 바이너리 크기 | ~13MB (웹 UI 내장) |
| 인덱싱 (55파일, 682청크) | < 2초 |
| BM25 검색 응답 | ~10ms |
| 시맨틱 검색 (72벡터) | ~5ms (벡터 비교) + Ollama RTT |
| 임베딩 생성 (72청크) | ~30초 (nomic-embed-text, 로컬) |
| 피크 메모리 | ~12MB |
| DB 크기 (72청크 + 벡터) | ~500KB |
