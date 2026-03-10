# CLAUDESTRA
## Claude Code Multi-Agent Orchestration System
### 기술 설계 문서 v2.2

| 항목 | 내용 |
|------|------|
| 프로젝트명 | Claudestra |
| MVP 언어 | Python |
| 최종 언어 | Go + React (Wails v2) |
| 대상 플랫폼 | Linux / Windows |
| AI 엔진 | Claude Code CLI |
| 에이전트 격리 | subprocess (현재) → Docker 컨테이너 (최종) |
| 팀장 도구 방식 | Claudestra CLI 바이너리 (v2.0 핵심) |
| 문서 버전 | 2.2.0 |

---

## v2.2 변경 사항 요약 — 5단계 코드 리팩토링

v2.1까지 누적된 구조적 문제(중복 코드, 거대 파일, 미사용 코드, 네이밍 불일치)를 5단계에 걸쳐 체계적으로 정리했습니다.

### 단계 1: BuildAgentsFromPlans 중복 제거

`app.go`의 `buildTeamFromPlans`와 `workspace.go`의 `BuildAgentsFromPlans`가 90% 동일한 코드였습니다.

**변경:**
- `BuildOptions` struct 도입 (`DetectWriteTool`, `LoadContract` 필드)
- `workspace.go`의 `BuildAgentsFromPlans(plans, opts)`를 유일한 팩토리로 통합
- `app.go`에서 `rebuildTeam()` 메서드가 `workspace.BuildAgentsFromPlans()` 호출
- 기존 `app.go:buildTeamFromPlans` 삭제

```go
// workspace.go — 통합 팩토리
type BuildOptions struct {
    DetectWriteTool bool  // consumer "문서"/"작성" 키워드 → Write 툴 추가
    LoadContract    bool  // contract 로딩 여부
}

func (w *Workspace) BuildAgentsFromPlans(plans []RolePlan, opts BuildOptions) map[string]*Agent { ... }

// app.go — 호출부
func (a *App) rebuildTeam(plans []internal.RolePlan) {
    a.agents = a.workspace.BuildAgentsFromPlans(plans, internal.BuildOptions{
        DetectWriteTool: true, LoadContract: true,
    })
}
```

### 단계 2: app.go 분할 (577줄 → 3개 파일)

| 파일 | 줄수 | 역할 |
|------|------|------|
| `app.go` | 301 | Wails RPC 바인딩, 프로젝트 관리, `RunLeadSession` |
| `app_watchers.go` | 122 (신규) | `startLogWatcher`, `startPermissionWatcher`, `startTeamWatcher` |
| `app_detail.go` | 164 (신규) | `AgentStatusInfo`/`AgentDetailInfo` DTO, `GetAgentDetail`, `GetAgentStatuses`, `GetLocks` |

- `RunLeadSession`은 `app.go`에 유지하되, 워처 시작은 `app_watchers.go`의 메서드 호출로 위임
- 프론트엔드용 DTO 타입(`AgentStatusInfo`, `AgentDetailInfo`)은 `app_detail.go`로 이동

### 단계 3: CLI main.go 분할 (715줄 → 5개 파일)

| 파일 | 줄수 | 포함 핸들러 |
|------|------|------------|
| `main.go` | 97 | `main()`, switch 라우팅, `mustWorkspace`/`mustPlans`/`printJSON` 헬퍼 |
| `cmd_team.go` | 117 | `cmdStatus`, `cmdTeam`, `cmdTeamSet` |
| `cmd_session.go` | 101 | `cmdSession`, `cmdIssues`, `cmdLeadSession` |
| `cmd_agent.go` | 267 | `cmdIdea`, `cmdOutput`, `cmdAssign`(Sync/Async), `cmdRunJob`, `cmdWait`, `cmdStop` |
| `cmd_config.go` | 138 | `cmdContract`, `cmdHook`, `cmdHookPreToolUse`, `describeToolCall` |

모두 `package main`이므로 파일 분리만으로 import 변경 없음.

### 단계 4: 미사용 코드 정리

- `ProposalPanel.tsx` 삭제 — 어디서도 import되지 않는 dead code (233줄)
- `buildLegacyTeam`은 `OpenProject`에서 여전히 사용되어 유지

### 단계 5: 네이밍 정리

`Workspace`에 경로 헬퍼 추가로 `filepath.Join` 반복 제거:

```go
func (w *Workspace) AgentDir(plan RolePlan) string {
    return filepath.Join(w.Root, plan.Directory)
}
func (w *Workspace) LogPath(role string) string {
    return filepath.Join(w.LogsDir, role+".jsonl")
}
```

`BuildAgentsFromPlans` 내부에서 `w.AgentDir(plan)`, `w.LogPath(plan.Role)` 사용.

---

## 코드 변경 범위 (v2.2)

| 파일 | 상태 | 변경 내용 |
|------|------|-----------|
| `gui/app.go` | 수정 | `buildTeamFromPlans` 삭제 → `rebuildTeam`이 `workspace.BuildAgentsFromPlans()` 호출. 워처 코드 분리. 577→301줄 |
| `gui/app_watchers.go` | **신규** | `startLogWatcher`, `startPermissionWatcher`, `startTeamWatcher` (122줄) |
| `gui/app_detail.go` | **신규** | `AgentStatusInfo`/`AgentDetailInfo` DTO, `GetAgentDetail`, `GetAgentStatuses`, `GetLocks` (164줄) |
| `gui/cmd/claudestra/main.go` | 수정 | 서브커맨드 핸들러 분리 → 라우팅만 남김. 715→97줄 |
| `gui/cmd/claudestra/cmd_team.go` | **신규** | `cmdStatus`, `cmdTeam`, `cmdTeamSet` (117줄) |
| `gui/cmd/claudestra/cmd_session.go` | **신규** | `cmdSession`, `cmdIssues`, `cmdLeadSession` (101줄) |
| `gui/cmd/claudestra/cmd_agent.go` | **신규** | `cmdIdea`, `cmdOutput`, `cmdAssign*`, `cmdRunJob`, `cmdWait`, `cmdStop` (267줄) |
| `gui/cmd/claudestra/cmd_config.go` | **신규** | `cmdContract`, `cmdHook`, `cmdHookPreToolUse`, `describeToolCall` (138줄) |
| `gui/internal/workspace.go` | 수정 | `BuildOptions` struct, `AgentDir`/`LogPath` 헬퍼 추가 (231→256줄) |
| `gui/frontend/src/components/ProposalPanel.tsx` | **삭제** | 미사용 dead code 제거 |

**총계**: 신규 6개, 수정 3개, 삭제 1개

---

## v2.1 변경 사항 (히스토리)

### 1. GUI UX 개선 — 서브에이전트 분리 표시

- **중앙 영역**: 팀장(Lead) 메시지만 표시. 서브에이전트 출력 제거
- **사이드바**: 실행 중(RUNNING) 에이전트에 파란색 글로우 효과
- **에이전트 상세 패널**: 탭 순서 변경 (받은 지시 → 실행 로그 → 실행 결과)

### 2. 텍스트 꼬임 해결

`log-append` 이벤트에 agent 식별 정보가 없어 텍스트가 섞이는 문제 → `LogEvent` 객체로 agent별 정확한 append 대상 매칭.

### 3. CLI `wait` 명령 추가

`claudestra wait <agent1> [agent2 ...]` — RUNNING→DONE/ERROR까지 블로킹 대기. sleep 폴링 제거.

### 4. 세션 메모리 재로드

`RunLeadSession` 시작 시 매번 디스크에서 `session.json` 재로드.

### 5. InitProject 팀 복원 / 에이전트 상태 디스크 읽기 / 로그 색상 통일

---

# 1. 프로젝트 개요

## 1.1 목표

Claudestra는 여러 Claude Code 인스턴스를 오케스트레이션하여 복잡한 소프트웨어 개발 작업을 자동화하는 멀티 에이전트 시스템입니다.

사용자는 팀장(Lead Agent)과 다양한 역할의 팀원(Sub-Agent) 노드를 구성하고,
팀장 AI가 사용자의 요구사항을 분석하여 각 팀원에게 최적화된 태스크를 배분합니다.

> **핵심 컨셉**: 팀장 AI = 오케스트레이터 / 팀원 AI = 전문 실행자.
> 각자 격리된 워크스페이스에서 작업하며 팀장이 Claudestra CLI 도구를 통해 전체를 조율합니다.

## 1.2 현재 구현 상태

| 단계 | 상태 |
|------|------|
| Phase 1 — Python MVP | ✅ 완료 |
| Phase 2 — 권한 제어 (`--allowedTools`) | ✅ 완료 |
| Phase 3 — Go + Wails GUI | ✅ 완료 |
| Phase 3.5 — CLI 도구화 | ✅ 완료 (v2.1) |
| Go 백엔드 코드 정리 | ✅ 완료 |
| **코드 리팩토링 (5단계)** | **✅ 완료 (v2.2)** |
| Phase 4 — Docker 격리 | 미착수 |

---

# 2. CLI 도구화 — v2.0 핵심 설계

## 2.1 기존 구조의 문제

현재 팀장(lead.go)의 Claude 호출 지점:

```
Go 코드 → callClaude (IsDevTask)          ← 항상 실행
Go 코드 → callClaude (PlanTeam)           ← 항상 실행
Go 코드 → callClaudeStream (Decompose)    ← 항상 실행
Go 코드 → callClaudeStream (Contract)     ← 항상 실행
Go 코드 → callClaudeTextOnly (Summary)    ← 항상 실행
Go 코드 → callClaudeTextOnly (RefreshProjectSummary)  ← 항상 실행
Go 코드 → callClaudeTextOnly (ExtractIssues)           ← consumer 있을 때
Go 코드 → callClaudeTextOnly (DirectReply)             ← 비개발 태스크
```

**문제:**
- 총 7~8회 호출, 매번 풀 컨텍스트 중복 주입
- Go 코드가 호출 순서를 하드코딩 → Claude의 판단 여지 없음
- 단순 인사에도 IsDevTask 호출 후 DirectReply 호출 = 최소 2회

## 2.2 새 구조 — Claude가 CLI 도구를 직접 호출

```
새 방식 — 팀장 Claude 단일 세션, 필요할 때만 CLI 호출
──────────────────────────────────────────────────────
Claude → $ claudestra plan "..."         ← 필요할 때만
Claude → $ claudestra assign backend     ← 필요할 때만
Claude → $ claudestra assign frontend    ← 필요할 때만
Claude → $ claudestra status             ← 필요할 때만
Claude → $ claudestra issues             ← 리뷰어 있을 때만
... 필요한 것만 호출, 컨텍스트 1회 주입
```

## 2.3 아키텍처 (v2.2 업데이트)

```
┌─────────────────────────────────────────────────────┐
│                   GUI Layer (Wails v2)               │
│  ┌──────────────┐                    ┌───────────┐  │
│  │  Sidebar      │                    │  LogPanel │  │
│  │  (agents)     │                    │ (stream)  │  │
│  └──────┬────────┘                    └─────┬─────┘  │
└─────────┼──────────────────────────────────┼────────┘
          │                                  │
┌─────────▼──────────────────────────────────▼────────┐
│            Lead Agent (Claude Code 단일 세션)          │
│                                                       │
│  사용자 입력 수신                                       │
│       │                                               │
│       ▼  Bash 도구로 Claudestra CLI 직접 호출           │
│  $ claudestra plan "..."                              │
│  $ claudestra assign backend "..."                    │
│  $ claudestra status                                  │
│  $ claudestra issues                                  │
│  $ claudestra session get                             │
└───────────────────────────┬───────────────────────────┘
                            │
┌───────────────────────────▼───────────────────────────┐
│          Claudestra CLI (Go 바이너리)                   │
│                                                       │
│  plan / assign / status / issues / session / contract │
└───────────────────────────┬───────────────────────────┘
                            │
         ┌──────────────────┴──────────────────┐
         │                                     │
┌────────▼────────┐                  ┌─────────▼───────┐
│  현재            │                  │  Phase 4         │
│  subprocess     │                  │  Docker          │
│  (claude CLI)   │                  │  컨테이너         │
└─────────────────┘                  └─────────────────┘
```

> v2.2 변경: 아키텍처 다이어그램에서 `ProposalPanel` 제거 (미사용 코드로 삭제됨)

## 2.4 MCP vs CLI 선택 근거

| | MCP 서버 | Claudestra CLI |
|--|----------|----------------|
| 구현 복잡도 | 높음 (서버 프로세스 관리) | 낮음 (바이너리 하나) |
| 디버깅 | 프로토콜 로그 분석 필요 | 터미널에서 직접 실행 |
| Claude 친화성 | 별도 프로토콜 학습 필요 | CLI 수백만 건 학습됨 |
| 조합성 | 서버 형식에 종속 | pipe, grep, jq 자유롭게 |
| 인증/안정성 | 초기화 불안정, 재인증 빈번 | 바이너리 실행, 상태 없음 |
| 인간 디버깅 | LLM 대화 내부에만 존재 | 사람도 동일 명령어 실행 가능 |

## 2.5 CLI 명령어 (구현 완료)

```bash
claudestra status                         # 팀원 전체 상태 출력
claudestra team                           # 팀원 목록 (JSON)
claudestra team set '<json>'              # 팀 구성 설정
claudestra session get                    # 세션 메모리 읽기
claudestra session update '<json>'        # 세션 메모리 갱신
claudestra issues                         # 미해결 이슈 목록 출력
claudestra contract get                   # 인터페이스 계약서 조회
claudestra contract set '<yaml>'          # 인터페이스 계약서 설정
claudestra idea    <agent>                # 에이전트 이데아 출력
claudestra output  <agent>                # 에이전트 최근 출력 조회
claudestra assign  <agent> '<instr>'      # 동기 작업 지시 (완료까지 대기)
claudestra assign --async <agent> '<instr>'  # 비동기 작업 지시 (job-id 반환)
claudestra wait <agent1> [agent2 ...]     # 비동기 작업 완료 대기 (v2.1)
claudestra stop <agent|job-id>            # 실행 중인 작업 중지
claudestra lead-session '<request>'       # 단일 세션 모드 (팀장 자율 실행)
```

### CLI 파일 구조 (v2.2)

```
gui/cmd/claudestra/
├── main.go          (97줄)  — main(), switch 라우팅, 공용 헬퍼
├── cmd_team.go     (117줄)  — cmdStatus, cmdTeam, cmdTeamSet
├── cmd_session.go  (101줄)  — cmdSession, cmdIssues, cmdLeadSession
├── cmd_agent.go    (267줄)  — cmdIdea, cmdOutput, cmdAssign*, cmdRunJob, cmdWait, cmdStop
└── cmd_config.go   (138줄)  — cmdContract, cmdHook, cmdHookPreToolUse, describeToolCall
```

## 2.6 팀장 세션 프롬프트 (`buildLeadSessionPrompt`)

`RunLeadSession` 호출 시 팀장 Claude에게 주입되는 프롬프트. `{CLI}` 플레이스홀더가 실제 바이너리 경로로 치환됩니다:

```
당신은 소프트웨어 개발 팀의 팀장 AI입니다.
Bash 도구를 통해 claudestra CLI로 팀을 관리하고 작업을 수행합니다.

[사용 가능한 CLI 명령어]
{CLI} team / status / session get|update / issues
{CLI} contract get|set / idea <agent> / output <agent>
{CLI} assign <agent> '<instruction>'         (동기)
{CLI} assign --async <agent> '<instruction>' (비동기)

[작업 흐름]
1. {CLI} team 으로 팀 구성 확인
2. {CLI} session get 으로 프로젝트 컨텍스트 파악
3. 사용자 요청 분석
4. 비개발 태스크 → 직접 응답
5. 개발 태스크:
   a. {CLI} idea <agent> 로 역할 확인
   b. {CLI} contract set 으로 계약서 설정
   c. {CLI} assign 으로 작업 지시
   d. 결과 취합 → 보고서 → {CLI} session update
```

## 2.7 두 실행 경로의 공존

```
경로 A (GUI 승인 플로우) — 기존 유지
    PlanRequest → 사용자 승인 → ExecutePlan
    lead.go의 Decompose/GenerateContract/summarize 등 사용
    Claude 호출: Lead 6~8회 + sub-agent N회

경로 B (단일 세션 모드) — v2.1 구현
    RunLeadSession → claudestra CLI → assign × N
    Claude 호출: Lead 1회 + sub-agent N회
```

---

# 3. 시스템 아키텍처 (v2.2 업데이트)

## 3.1 프로젝트 구조

```
Claudestra/
├── gui/                              ← Go + Wails GUI (현재)
│   ├── main.go                       ← Wails 앱 진입점
│   ├── app.go                        ← Wails RPC 바인딩, 프로젝트 관리, RunLeadSession (301줄)
│   ├── app_watchers.go               ← fsnotify 워처: 로그, 권한, 팀 (122줄, v2.2 신규)
│   ├── app_detail.go                 ← 에이전트 상세 DTO, GetAgentDetail/Statuses (164줄, v2.2 신규)
│   │
│   ├── cmd/claudestra/               ← claudestra CLI 바이너리 (v2.2 분할)
│   │   ├── main.go                   ← 서브커맨드 라우팅 (97줄)
│   │   ├── cmd_team.go               ← team, status (117줄)
│   │   ├── cmd_session.go            ← session, issues, lead-session (101줄)
│   │   ├── cmd_agent.go              ← assign, wait, stop, output, idea (267줄)
│   │   └── cmd_config.go             ← contract, hook (138줄)
│   │
│   ├── internal/
│   │   ├── lead.go                   ← 팀장 에이전트 (Decompose, Contract, RunLeadSession 등)
│   │   ├── agent.go                  ← 팀원 에이전트 (subprocess 래핑, 스트리밍, JSONL 듀얼 라이트)
│   │   ├── streamparser.go           ← 공유 Claude CLI stream-json 파서
│   │   ├── workspace.go              ← 워크스페이스 관리, BuildAgentsFromPlans 통합 팩토리 (256줄)
│   │   ├── filelock.go               ← 파일 락 레지스트리
│   │   ├── permissions.go            ← 도구 화이트리스트 + GUI 권한
│   │   ├── jobs.go                   ← Job CRUD (비동기 assign용)
│   │   └── logwatcher.go             ← fsnotify 기반 JSONL 로그 감시
│   │
│   └── frontend/src/
│       ├── App.tsx                   ← 메인 앱 (상태 관리, 이벤트 수신)
│       └── components/
│           ├── LogPanel.tsx           ← 로그 패널 (thinking 그룹, 스트리밍)
│           ├── Sidebar.tsx            ← 에이전트 목록
│           ├── InputBar.tsx           ← 사용자 입력
│           ├── ReportPanel.tsx        ← 최종 보고서
│           ├── AgentDetailPanel.tsx   ← 에이전트 상세 정보
│           ├── PermissionDialog.tsx   ← 도구 승인 다이얼로그
│           ├── MarkdownRenderer.tsx   ← 마크다운 렌더링
│           └── ProjectSetup.tsx       ← 프로젝트 초기화
│
└── docs/                             ← 기술 문서
```

> v2.2 변경: `ProposalPanel.tsx` 삭제, `app_watchers.go`/`app_detail.go` 신규, CLI `cmd_*.go` 분할

## 3.2 Go 백엔드 줄수 비교 (리팩토링 전후)

| 파일 | 리팩토링 전 | 리팩토링 후 | 비고 |
|------|------------|------------|------|
| `app.go` | 577 | 301 | 워처/상세 분리 |
| `app_watchers.go` | — | 122 | 신규 |
| `app_detail.go` | — | 164 | 신규 |
| `cmd/claudestra/main.go` | 715 | 97 | 핸들러 분리 |
| `cmd/claudestra/cmd_team.go` | — | 117 | 신규 |
| `cmd/claudestra/cmd_session.go` | — | 101 | 신규 |
| `cmd/claudestra/cmd_agent.go` | — | 267 | 신규 |
| `cmd/claudestra/cmd_config.go` | — | 138 | 신규 |
| `workspace.go` | 231 | 256 | BuildOptions, 헬퍼 추가 |

**총 줄수**: 1,523→1,563줄 (미사용 코드 제거 후 실질 변화 미미, 구조만 개선)

## 3.3 워크스페이스 디렉토리 구조

```
ProjectDir/                       ← 팀장(Lead) playground
│
├── .orchestra/                   ← Claudestra 내부 설정
│   ├── config.yaml               ← 프로젝트 설정
│   ├── ideas/                    ← 에이전트별 이데아
│   ├── session.json              ← 프로젝트 메모리 (세션 상태)
│   ├── team.json                 ← 동적 팀 구성 정보
│   ├── contracts/
│   │   └── contract.yaml         ← 인터페이스 계약서
│   ├── locks/                    ← 파일 락 레지스트리
│   ├── jobs/                     ← 비동기 assign job 파일
│   │   └── job-{id}.json
│   └── logs/                     ← 에이전트 JSONL 실시간 로그
│       └── {agent}.jsonl
│
├── vision_dev/                   ← Producer 팀원 playground
│   └── .agent-status
│
└── build_qa/                     ← Consumer 팀원 playground
    └── .agent-status
```

## 3.4 에이전트 유형 분류

| 유형 | 예시 역할 | 허용 도구 | 크로스 참조 |
|------|-----------|-----------|------------|
| Producer | developer, vision_dev | Read, Write, Edit, Glob, Grep, Bash | 없음 |
| Consumer | reviewer, build_qa | Read, Glob, Grep (+ Write 선택적) | 타 Producer dirs 읽기 전용 |
| Lead | 팀장 | Read, Glob, Grep, Bash (Claudestra CLI 포함) | 전체 |

---

# 4. 실시간 스트리밍 아키텍처

## 4.1 Claude CLI stream-json 와이어 포맷

`claude -p --output-format stream-json --include-partial-messages --verbose` 명령의 실제 출력:

```
{"type":"system","subtype":"init","model":"...","tools":[...]}
{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"thinking"}}}
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"분석 중..."}}}
{"type":"stream_event","event":{"type":"content_block_stop","index":0}}
{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"text"}}}
{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"응답 텍스트..."}}}
{"type":"result","subtype":"success","result":"최종 텍스트","duration_ms":...}
```

## 4.2 공유 스트림 파서 (streamparser.go)

```go
type StreamCallbacks struct {
    OnText     func(text string)                   // 단어 단위 flush
    OnThinking func(text string)                   // thinking 단어 단위 flush
    OnToolUse  func(toolName string, input string) // 도구 호출 요약
    OnResult   func(result string)                 // 최종 결과
}

func ParseStream(reader io.Reader, cb StreamCallbacks)
```

## 4.3 이벤트 흐름

```
Claude CLI stdout
    │
    ▼
ParseStream (streamparser.go)
    │
    ├─ OnThinking("분석 중...")  → logFn("💭 분석 중...")
    ├─ OnToolUse("Read", "file.go") → logFn("🔧 Read: file.go")
    ├─ OnText("응답 텍스트...")  → logFn("응답 텍스트...")
    └─ OnResult("최종")         → fullResult = "최종"
                                         │
                                         ▼
                              app_watchers.go 워처 콜백
                                         │
                              "💭"/"🔧"/"📊" → LogEvent{type:"thinking"}
                              그 외         → LogEvent{type:"text"}
                              "\x01" 접두사  → "log-append" 이벤트
                                         │
                                         ▼
                              LogPanel (thinking 그룹 독립 토글 렌더링)
```

## 4.4 프론트엔드 성능 최적화

| 문제 | 해결 |
|------|------|
| 토큰마다 setState → 초당 수백 회 리렌더링 | `requestAnimationFrame` 배치 → 최대 60회/초 |
| `scrollIntoView({ behavior: 'smooth' })` 큐 폭발 | `behavior: 'auto'` + 사용자 스크롤 시 비활성화 |
| 전체 LogPanel 매번 리렌더링 | `React.memo` 적용 |

---

# 5. 에이전트 메모리 시스템

## 5.1 구조화된 세션 메모리

```json
// .orchestra/session.json
{
  "project_summary": "카메라 캘리브레이션 시각화 모듈 복구 중",
  "completed_tasks": [
    "vision_visualization_dev: zhangVisualization.h/cpp 파일 복구 완료"
  ],
  "open_issues": [
    {
      "id": "issue-001",
      "found_by": "build_qa",
      "severity": "high",
      "description": "컴파일 에러: DrawLine 함수 시그니처 불일치",
      "status": "open"
    }
  ],
  "recent_conversations": [
    {"role": "user", "content": "시각화 파일 복구해줘"},
    {"role": "lead", "content": "vision_visualization_dev에게 복구 태스크 배분 완료"}
  ]
}
```

## 5.2 토큰 관리 전략

| 항목 | 유지 방법 | 토큰량 |
|------|-----------|--------|
| `project_summary` | 매 작업 후 1~2문장으로 갱신 | 소량 |
| `completed_tasks` | 최근 20개만 요약 보관 | 소량 |
| `open_issues` | 해결되면 `status: resolved` | 중간 |
| `recent_conversations` | 마지막 10턴, 200자 truncate | 제한적 |

---

# 6. 에이전트 권한 & 격리 전략

## 6.1 현재 (Phase 3) — `--allowedTools` + 디렉토리 격리

```go
// agent.go
args = append(args, "--allowedTools", strings.Join(a.Config.AllowedTools, ","))
cmd.Dir = a.Config.WorkDir  // 자기 디렉토리로 격리
```

## 6.2 Phase 4 — Docker 컨테이너 격리

```go
// Phase 4: agent.go의 Start() 내부만 교체
cmd := exec.Command("docker", "run", "--rm",
    "-v", agent.WorkDir+":/workspace",
    "-v", "./.orchestra:/orchestra:ro",
    "--network", "none", "--memory", "2g", "--cpus", "1.0",
    "claude-agent:latest", "claude", "--print", prompt,
)
```

---

# 7. 이데아(Idea) 시스템

각 에이전트의 역할, 전문성, 행동 방침을 정의하는 시스템 프롬프트.

```yaml
role: vision_visualization_dev
idea: |
  당신은 로봇 비전 시각화 개발 전문가입니다.
  담당 디렉토리: ./vision_visualization_dev/
  전문 영역:
  - OpenCV / Qt 기반 이미지 시각화
  - 카메라 캘리브레이션 결과 렌더링
  작업 규칙:
  - 자신의 디렉토리 외부 파일은 절대 수정하지 않습니다.
  - 인터페이스 계약서의 명세를 따릅니다.
```

---

# 8. 네이밍 규칙 (v2.2 확정)

| 범주 | 규칙 | 예시 |
|------|------|------|
| 파일명 | snake_case, 역할 기반 | `app_watchers.go`, `cmd_team.go` |
| 타입명 | PascalCase | `Agent`, `RolePlan`, `AgentStatusInfo` |
| 프론트엔드 DTO | `~Info` 접미사 (위치: `app_detail.go`) | `AgentStatusInfo`, `AgentDetailInfo` |
| 내부 도메인 | 접미사 없음 (위치: `internal/`) | `Agent`, `AgentStatus`, `Session` |
| CLI 핸들러 | `cmd~` 접두사 | `cmdTeam()`, `cmdAssign()` |
| 워처 메서드 | `start~Watcher` | `startLogWatcher()`, `startTeamWatcher()` |
| 경로 헬퍼 | `Workspace` 메서드 | `AgentDir(plan)`, `LogPath(role)` |

---

# 9. 개발 단계 로드맵

```
Phase 1  Python MVP          오케스트레이션 핵심 로직 검증          ✅ 완료
Phase 2  권한 제어 강화       --allowedTools 도구 제한 추가          ✅ 완료
Phase 3  Go + Wails GUI       검증된 로직을 Go로 포팅 + GUI         ✅ 완료
  ├── Go 포팅 + React GUI                                          ✅ 완료
  ├── 세션 메모리 (session.json)                                    ✅ 완료
  ├── 동적 팀 구성 (PlanTeam)                                       ✅ 완료
  ├── Plan → Approve → Execute GUI 워크플로우                       ✅ 완료
  ├── 실시간 스트리밍 + Thinking 표시                                ✅ v1.4
  └── 독립 thinking 토글 + 분석 텍스트 그룹화                        ✅ v1.5
Phase 3.5  CLI 도구화          팀장 Claude → Claudestra CLI          ✅ v2.1 완료
  ├── [✅] Phase A: 읽기 전용 CLI
  ├── [✅] Phase B: 동기 assign + JSONL 듀얼 라이트
  ├── [✅] Phase C: 비동기 assign + job 관리
  ├── [✅] Phase D: Lead 단일 세션
  └── [✅] Phase E: GUI fsnotify 통합
Phase 3.6  코드 리팩토링       구조 개선, 중복 제거, 파일 분할        ✅ v2.2 완료
  ├── [✅] 단계 1: BuildAgentsFromPlans 중복 제거
  ├── [✅] 단계 2: app.go 분할 (577→3파일)
  ├── [✅] 단계 3: CLI main.go 분할 (715→5파일)
  ├── [✅] 단계 4: 미사용 코드 정리 (ProposalPanel.tsx 삭제)
  └── [✅] 단계 5: 네이밍 정리 (AgentDir/LogPath 헬퍼)
Phase 4  Docker 격리          컨테이너 기반 OS 레벨 샌드박스
Phase 5  Docker Compose       전체 스택 선언적 관리
```

---

# 10. 주요 도전 과제 & 해결 전략

| 도전 과제 | 난이도 | 해결 전략 | 상태 |
|-----------|--------|-----------|------|
| Claude Code stdout 파싱 | 높음 | `stream-json` + `--include-partial-messages` + 공유 파서 | ✅ 해결 |
| 작업 완료 감지 신뢰성 | 중간 | `.agent-status` 파일 + 5분 타임아웃 | ✅ 해결 |
| 팀원 간 파일 충돌 | 중간 | 디렉토리 격리 + FileLockRegistry + 이데아 규칙 | ✅ 해결 |
| 토큰 폭발 | 높음 | CLI 도구화 + session.json 경량 메모리 | ✅ v2.1 |
| 팀장 태스크 분해 품질 | 높음 | 구조화된 프롬프트 + JSON 출력 강제 + 폴백 로직 | ✅ 해결 |
| **코드 구조 복잡성** | **중간** | **5단계 리팩토링 — 파일 분할, 중복 제거, 네이밍 통일** | **✅ v2.2** |
| Docker 내 Claude 인증 | 중간 | API 키 환경변수 주입 전략 | Phase 4 |

---

# 11. 기술 의존성

**Go:**

| 패키지 | 용도 |
|--------|------|
| `github.com/wailsapp/wails/v2` | GUI 프레임워크 |
| `gopkg.in/yaml.v3` | 이데아/설정 YAML |
| `github.com/fsnotify/fsnotify` | JSONL 로그 파일 감시 |

**프론트엔드:**

| 패키지 | 용도 |
|--------|------|
| React + TypeScript | UI 프레임워크 |

---

# 12. 미결 사항

- [x] CLI 도구화 — `claudestra` 바이너리 설계 및 구현 ✅ v2.1
- [x] CLI assign 비동기 실행 — self-re-exec + Setsid detach ✅ v2.1
- [x] GUI ↔ CLI 스트리밍 연동 — fsnotify LogWatcher + JSONL 듀얼 라이트 ✅ v2.1
- [x] **코드 리팩토링 — 5단계 구조 개선 완료** ✅ v2.2
- [x] **BuildAgentsFromPlans 중복 제거** ✅ v2.2
- [x] **app.go / CLI main.go 파일 분할** ✅ v2.2
- [x] **미사용 코드 정리 (ProposalPanel.tsx)** ✅ v2.2
- [x] **네이밍 정리 (AgentDir/LogPath 헬퍼)** ✅ v2.2
- [ ] 경로 A 토큰 최적화 — GUI 승인 플로우의 Decompose+GenerateContract 통합
- [ ] Docker 이미지 내 Claude Code 인증 처리 방법
- [ ] 에이전트 비용 추적 (Claude Code 사용량 모니터링)
- [ ] 로그 가상화 — 장시간 세션에서 수천 개 로그 항목 렌더링 성능 (react-window 등)
