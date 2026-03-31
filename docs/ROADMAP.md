# Ledger Roadmap

## 현재 진행 상황

```
Phase 1  ██████████ 완료 (DocumentPipe + BM25 + CLI)
Phase 2  ██████████ 완료 (벡터 임베딩 + 시맨틱 검색)
Phase 3  ██████████ 완료 (VCS hook 자동 인덱싱)
Phase 4  ██████████ 완료 (버전 스냅샷 + diff)
Phase 5  ██████████ 완료 (모순 감지)
Phase 6  ██████████ 완료 (역할별 검색 우선순위)
Phase 7  ██████████ 완료 (JSON API + Node.js SDK)
Phase 8  ██████████ 완료 (플러그인 시스템)
Phase 9  ██████████ 완료 (웹 UI)
```

---

## Phase 1: DocumentPipe + BM25 + CLI (완료)

- [x] `.ledger.toml` 설정 파일 파싱
- [x] MD 파일 파싱 + 헤딩 기반 청킹
- [x] 파일 유형 자동 감지 (changelog, issues, rules 등)
- [x] 자동 태깅 (security, auth, api 등)
- [x] SQLite + FTS5 인덱싱
- [x] BM25 키워드 검색
- [x] 필터: --type, --tag, --after, --before, --top
- [x] 증분 업데이트 (파일 해시 비교)
- [x] CLI: `ledger index`, `ledger search`, `ledger status`
- [x] 출처 표시 (파일:라인 + 헤딩 + 미리보기)

## Phase 2: 벡터 임베딩 + 시맨틱 검색 (완료)

- [x] Ollama API 클라이언트 (`POST /api/embeddings`)
- [x] 청크 임베딩 생성 + vectors 테이블 BLOB 저장
- [x] 코사인 유사도 벡터 검색 (Go in-memory)
- [x] 하이브리드 스코어링 (정규화 BM25 + 코사인, alpha=0.5)
- [x] `ledger embed [--model] [--endpoint]` 명령
- [x] `ledger search --mode keyword|semantic|hybrid`
- [x] 연속 실패 시 early exit

## Phase 3: VCS hook 자동 인덱싱 (완료)

- [x] `internal/hook/` — 마커 기반 hook 삽입/제거
- [x] `ledger hook install|uninstall|status`
- [x] `ledger init` 시 git repo면 자동 설치
- [x] `ledger index --quiet` 백그라운드 실행

## Phase 4: 버전 스냅샷 + diff (완료)

- [x] snapshots + snapshot_chunks 테이블
- [x] `ledger snapshot [--label]` — 현재 인덱스 스냅샷
- [x] `ledger snapshot list` — 스냅샷 목록
- [x] `ledger diff <ref-a> <ref-b>` — content_hash 기반 비교
- [x] `ledger restore <ref>` — 시점 복원
- [x] git ref 지원 (HEAD, HEAD~N, commit hash)

## Phase 5: 모순 감지 (완료)

- [x] 벡터 유사도 기반 모순 탐지 (threshold 설정 가능)
- [x] 날짜 기반 superseded 판정
- [x] 벡터 없으면 헤딩 중복 기반 폴백
- [x] `ledger check [--fix] [--threshold N]`

## Phase 6: 역할별 검색 우선순위 (완료)

- [x] `--role` 플래그 (developer, ios_developer, director, security, qa)
- [x] 역할별 file_type 가중치 맵
- [x] 점수 부스트 후 재정렬

## Phase 7: JSON API + Node.js SDK (완료)

- [x] REST JSON API 서버 (5 엔드포인트, CORS)
- [x] `ledger serve [--port N]`
- [x] Node.js SDK (`sdk/node/`) — LedgerClient 클래스
- [x] TypeScript 타입 정의 (index.d.ts)

## Phase 8: 플러그인 시스템 (완료)

- [x] 커스텀 파일 유형 (`[plugins.file_types.NAME]`)
- [x] 커스텀 태그 룰 (`[[plugins.tag_rules]]`)
- [x] 청킹 전략 선택 (`chunk_by = heading|paragraph|separator`)
- [x] `ParseFileWithOptions` — 플러그인 설정 전달

## Phase 9: 웹 UI (완료)

- [x] Go embed 기반 SPA
- [x] `ledger ui [--port N]` → localhost 브라우저 오픈
- [x] 검색 탭 (모드/역할/필터 지원)
- [x] 브라우즈 탭 (청크 목록 조회)
- [x] 체크 탭 (모순 감지 대시보드)
- [x] 상태바 (파일/청크/임베딩/모델)
