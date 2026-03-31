# Changelog

## 2026-03-31

### Phase 9 완료 — 웹 UI
- `internal/ui/` — Go embed 기반 SPA 웹 UI
- `index.html` — 검색/브라우즈/체크 3탭 대시보드
- 다크 테마 UI, 실시간 상태바
- `ledger ui [--port N]` 명령 추가

### Phase 8 완료 — 플러그인 시스템
- `config.go` — `[plugins]` 섹션 추가
- 커스텀 파일 유형: `[plugins.file_types.NAME]` pattern 매칭
- 커스텀 태그 룰: `[[plugins.tag_rules]]` match/tag
- 청킹 전략 선택: `chunk_by = "heading" | "paragraph" | "separator"`
- `docpipe/parser.go` — `ParseFileWithOptions`, 3가지 청킹 전략, 커스텀 태그 적용

### Phase 7 완료 — JSON API + Node.js SDK
- `internal/api/server.go` — REST JSON API 서버 (5 엔드포인트)
- `/api/search`, `/api/status`, `/api/chunks`, `/api/check`, `/api/health`
- CORS 지원
- `ledger serve [--port N]` 명령 추가
- `sdk/node/` — Node.js 클라이언트 (index.js + index.d.ts + package.json)

### Phase 6 완료 — 역할별 검색 우선순위
- `search.go` — `RoleWeights` 맵 (developer, ios_developer, director, security, qa)
- `ApplyRoleBoost` — 파일 유형별 가중치 곱 → 재정렬
- `--role` 플래그 추가

### Phase 5 완료 — 모순 감지
- `internal/check/check.go` — 벡터 유사도 기반 모순 감지
- 같은 주제 + 다른 파일 + 높은 유사도 → Contradiction
- 날짜 차이 → Superseded 판정
- 벡터 없으면 헤딩 중복 기반 폴백
- `ledger check [--fix] [--threshold N]` 명령 추가

### Phase 4 완료 — 버전 스냅샷 + diff
- `internal/snapshot/snapshot.go` — 스냅샷 생성/목록/비교/복원
- `snapshots` + `snapshot_chunks` 테이블 추가
- `ledger snapshot [--label]`, `ledger snapshot list` 명령
- `ledger diff <ref-a> <ref-b>` — content_hash 기반 추가/삭제 비교
- `ledger restore <ref>` — 스냅샷 시점으로 인덱스 복원
- git ref 지원 (HEAD, HEAD~N, commit hash)

### Phase 3 완료 — VCS hook 자동 인덱싱
- `internal/hook/hook.go` — post-commit hook 관리
- 마커 기반 삽입/제거 (기존 hook과 공존)
- `ledger hook install|uninstall|status` 명령 추가
- `ledger init` 시 git repo면 hook 자동 설치
- `--quiet` 플래그 추가 (hook에서 무음 실행용)

### Phase 2 완료 — 벡터 임베딩 + 시맨틱 검색
- `internal/embed/ollama.go` — Ollama HTTP 클라이언트 (Embed, EmbedBatch, Ping)
- `internal/store/vectors.go` — 벡터 BLOB 저장, 코사인 유사도, CRUD
- `vectors` 테이블 추가 (chunk_id, embedding BLOB, model)
- `search.go` — `SemanticSearch` (코사인 유사도), `HybridSearch` (BM25 + 코사인 정규화)
- `--mode keyword|semantic|hybrid` 플래그 추가
- `ledger embed [--model] [--endpoint]` 명령 추가
- 기본 모델: nomic-embed-text (768차원)
- status에 임베딩 수 + 모델명 표시
- 연속 3회 실패 시 early exit

### Phase 1 보강
- `**/*.md` 재귀 탐색 수정 (filepath.WalkDir 기반, 무한 depth 지원)
- `ledger init` 명령 추가 — .ledger.toml 자동 생성
- CLI Company 재테스트: 55파일, 682청크 인덱싱 성공

### Phase 1 완료 — DocumentPipe + BM25 + CLI
- Go 프로젝트 초기화 (go.mod, 디렉토리 구조)
- `internal/config` — `.ledger.toml` 파싱, glob 패턴 파일 수집
- `internal/docpipe` — MD 파일 파싱, 헤딩 기반 청킹, 파일 유형 감지, 자동 태깅
- `internal/store` — SQLite 초기화, chunks/metadata 테이블, FTS5 가상 테이블, 증분 업데이트
- `internal/search` — FTS5 BM25 검색, 필터(type/tag/date)
- `cmd/ledger` — CLI 엔트리포인트 (index, search, status 명령)
- CLI Company에서 테스트: 32파일, 137청크 인덱싱 + 검색 동작 확인
- 바이너리 7MB, 검색 10ms, 메모리 ~8MB

## 2026-03-30

### 프로젝트 생성
- Ledger 이름 확정 (구 ProjectMemory)
- 기능명세서 v1.0 작성 (cli-company에서 이관)
- 코딩 모델 전략 문서 작성 (4개 프로젝트 공유)
- DocumentPipe 아키텍처 설계 (코드용 CodePipe는 별도 프로젝트로 분리 결정)
