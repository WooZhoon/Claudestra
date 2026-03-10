# Claudestra

Claude Code 기반 멀티 에이전트 오케스트레이션 시스템.

여러 Claude Code 인스턴스가 팀을 이뤄 소프트웨어 개발 작업을 수행합니다.
팀장(Lead)이 요구사항을 분석하고, 팀원(Sub-Agent)에게 작업을 배분하고, 결과를 취합합니다.

---

## 설치 (Docker 빌드)

Go, Node, Wails 설치 필요 없이 Docker만 있으면 됩니다.

### 사전 준비

- Docker
- Claude Code CLI 인증 완료 (`npm i -g @anthropic-ai/claude-code && claude`)
- Linux: `sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0` (런타임 라이브러리)

### 설치

```bash
git clone https://github.com/WooZhoon/Claudestra.git
cd Claudestra
bash install.sh
```

### 실행

```bash
claudestra-gui
```

---

## 로컬 빌드 (Docker 없이)

<details>
<summary>Go + Node + Wails 직접 설치</summary>

### 필요 도구

- Go 1.23+
- Node.js 18+
- Wails v2: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Linux: `sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev`
- Claude Code CLI: `npm install -g @anthropic-ai/claude-code`

### 빌드 & 실행

```bash
cd gui
go install ./cmd/claudestra   # CLI 도구 설치
make build                     # GUI 빌드
./build/bin/gui                # 실행
```

개발 모드: `make dev`

</details>

---

## 사용법

1. GUI가 열리면 프로젝트 폴더를 선택합니다
2. 하단 입력창에 요청을 입력합니다 (예: "틱택토 게임 만들어줘")
3. 팀장이 자동으로 팀을 구성하고 작업을 배분합니다
4. 좌측 사이드바에서 팀원 상태를 확인하고, 클릭하면 상세 정보를 볼 수 있습니다
5. 완료 후 "보고서" 버튼으로 결과를 확인합니다

---

## 작동 원리

```
사용자 요청
    │
    ▼
┌──────────┐
│  팀장 AI  │  ← Claude Code 세션 1개
│  (Lead)   │
└────┬─────┘
     │  claudestra CLI로 팀 관리
     │
     ├── team set     → 팀원 구성
     ├── contract set → 인터페이스 계약서
     ├── assign       → 작업 지시 (동기/비동기)
     ├── wait         → 비동기 작업 완료 대기
     │
     ▼
┌──────────┐  ┌──────────┐  ┌──────────┐
│ 팀원 A   │  │ 팀원 B   │  │ 팀원 C   │  ← 각각 별도 Claude Code
│(Producer)│  │(Producer)│  │(Consumer)│
└──────────┘  └──────────┘  └──────────┘
     │              │              │
     └──────────────┴──────────────┘
                    │
                    ▼
              최종 보고서
```

- **Producer**: 코드를 작성하는 팀원 (Read, Write, Edit, Bash)
- **Consumer**: 결과물을 분석하는 팀원 (Read 전용, 리뷰어/QA)

---

## 기술 스택

| 레이어 | 기술 |
|--------|------|
| GUI | Wails v2 (Go + WebView) |
| 프론트엔드 | React 18 + TypeScript + Vite |
| 백엔드 | Go 1.23 |
| AI 엔진 | Claude Code CLI (stream-json) |
| 에이전트 격리 | subprocess + WorkDir + 파일 락 |
| 빌드/배포 | Docker 멀티스테이지 빌드 |

---

## CLI 명령어

팀장 AI가 Bash 도구를 통해 호출하는 `claudestra` CLI 명령어:

```bash
claudestra team                           # 팀원 목록 (JSON)
claudestra team set '<json>'              # 팀 구성 설정
claudestra status                         # 팀원 전체 상태 출력
claudestra assign  <agent> '<instr>'      # 동기 작업 지시 (완료까지 대기)
claudestra assign --async <agent> '<instr>'  # 비동기 작업 지시 (job-id 반환)
claudestra wait <agent1> [agent2 ...]     # 비동기 작업 완료 대기
claudestra session get|update             # 세션 메모리 읽기/갱신
claudestra contract get|set '<yaml>'      # 인터페이스 계약서 조회/설정
claudestra idea    <agent>                # 에이전트 이데아 출력
claudestra output  <agent>                # 에이전트 최근 출력 조회
claudestra issues                         # 미해결 이슈 목록 출력
```

---

## 프로젝트 구조

```
gui/
├── app.go                (301줄) ← Wails RPC 바인딩, RunLeadSession
├── app_watchers.go       (122줄) ← fsnotify 워처 (로그, 권한, 팀)
├── app_detail.go         (164줄) ← 에이전트 상세 DTO, GetAgentDetail
│
├── cmd/claudestra/                ← CLI 바이너리 (v2.2 분할)
│   ├── main.go           (97줄)  ← switch 라우팅, 공용 헬퍼
│   ├── cmd_team.go      (117줄)  ← cmdStatus, cmdTeam, cmdTeamSet
│   ├── cmd_session.go   (101줄)  ← cmdSession, cmdIssues, cmdLeadSession
│   ├── cmd_agent.go     (267줄)  ← cmdAssign, cmdWait, cmdStop, cmdOutput, cmdIdea
│   └── cmd_config.go    (138줄)  ← cmdContract, cmdHook
│
├── internal/
│   ├── lead.go                   ← 팀장 에이전트
│   ├── agent.go                  ← 팀원 에이전트 (subprocess 래핑)
│   ├── workspace.go     (256줄)  ← BuildAgentsFromPlans 통합 팩토리
│   ├── streamparser.go           ← Claude CLI stream-json 파서
│   ├── filelock.go               ← 파일 락 레지스트리
│   ├── permissions.go            ← 도구 화이트리스트
│   ├── jobs.go                   ← Job CRUD (비동기 assign)
│   └── logwatcher.go             ← fsnotify JSONL 로그 감시
│
└── frontend/src/                 ← React 18 + TypeScript
```

---

## 문서

- [기술설계문서 v2.2](docs/Claudestra_기술설계문서_v2.2.md) — 아키텍처, CLI 설계, 리팩토링 상세
