# 🎼 ORCHESTRA
## Claude Code Multi-Agent Orchestration System
### 기술 설계 문서 v1.1

| 항목 | 내용 |
|------|------|
| 프로젝트명 | Orchestra |
| MVP 언어 | Python |
| 최종 언어 | Go + React (Wails v2) |
| 대상 플랫폼 | Linux / Windows |
| AI 엔진 | Claude Code CLI |
| 에이전트 격리 | subprocess (MVP) → Docker 컨테이너 (최종) |
| 문서 버전 | 1.1.0 |

---

# 1. 프로젝트 개요

## 1.1 목표

Orchestra는 여러 Claude Code 인스턴스를 오케스트레이션하여 복잡한 소프트웨어 개발 작업을 자동화하는 멀티 에이전트 시스템입니다.

사용자는 팀장(Lead Agent)과 다양한 역할의 팀원(Sub-Agent) 노드를 구성하고, 팀장 AI가 사용자의 요구사항을 분석하여 각 팀원에게 최적화된 태스크를 배분합니다.

> 💡 **핵심 컨셉**: 팀장 AI = 오케스트레이터 / 팀원 AI = 전문 실행자.
> 각자 격리된 워크스페이스에서 작업하며 팀장이 전체를 조율합니다.

## 1.2 개발 전략 — 단계별 접근

이 프로젝트의 가장 큰 불확실성은 **"Claude Code subprocess를 실제로 안정적으로 제어할 수 있냐"** 입니다.
GUI나 언어 포팅은 그 이후 문제입니다. 따라서 검증 우선 전략을 취합니다.

```
Phase 1  Python MVP          오케스트레이션 핵심 로직 검증
Phase 2  권한 제어 강화       --allowedTools 도구 제한 추가
Phase 3  Go + Wails GUI       검증된 로직을 Go로 포팅 + GUI
Phase 4  Docker 격리          컨테이너 기반 진짜 샌드박스
Phase 5  Docker Compose       전체 스택 통합 관리
```

## 1.3 주요 특징

- Claude Code CLI를 subprocess로 래핑하여 각 에이전트 독립 실행
- React Flow 기반 노드 에디터로 팀 구성 시각화 (Phase 3~)
- 역할별 **이데아(Idea)** 시스템 — 사용자 편집 가능한 시스템 프롬프트
- 디렉토리 격리로 팀원 간 워크스페이스 충돌 방지
- fsnotify 기반 파일 변경 감지 및 작업 완료 신호 처리
- 코드 리뷰어, 문서 작성자 등 크로스 참조(Consumer) 에이전트 지원
- **최종적으로 Docker 컨테이너 기반 OS 레벨 격리** 적용

## 1.4 기술 스택 요약

| 레이어 | MVP (Python) | 최종 (Go) |
|--------|-------------|-----------|
| 언어 | Python 3.11+ | Go 1.22+ |
| GUI | 없음 (CLI) | Wails v2 + React Flow |
| 에이전트 실행 | subprocess + `--dangerously-skip-permissions` | Docker 컨테이너 |
| 상태 관리 | threading + `.agent-status` 파일 | goroutine + channel |
| 파일 감시 | polling / `.agent-status` | fsnotify |
| 이데아 저장 | YAML | YAML |
| 의존성 | pyyaml | wails, creack/pty, fsnotify |

---

# 2. 에이전트 권한 & 격리 전략

이 프로젝트에서 가장 중요한 설계 결정 중 하나입니다.
에이전트가 의도치 않게 다른 팀원의 파일이나 시스템을 건드리는 것을 어떻게 막느냐의 문제입니다.

## 2.1 단계별 격리 전략

```
Phase 1 (MVP)           Phase 2                 Phase 4 (최종)
────────────────────────────────────────────────────────────────
--dangerously-         --allowedTools로         Docker 컨테이너
skip-permissions       허용 도구 명시           OS 레벨 격리
+ 디렉토리 격리        + 파일 접근 경로 제한
+ 이데아 규칙
```

## 2.2 Phase 1 — skip-permissions + 디렉토리 격리

MVP 단계에서는 `--dangerously-skip-permissions` 플래그로 permission 프롬프트를 건너뜁니다.
대신 두 가지로 리스크를 관리합니다.

**왜 permission 릴레이 방식은 안 쓰나?**

permission 릴레이란 에이전트의 permission 프롬프트를 팀장이 캡처해서 사용자에게 전달하고, 응답을 다시 주입하는 방식입니다.
팀원이 5명이면 동시다발로 permission이 터지고, UX도 엉망이 되며, 구현 복잡도만 높아집니다. 실용적이지 않습니다.

**대신 이 두 가지로 충분합니다:**

1. **WorkDir 격리** — 각 에이전트의 작업 디렉토리를 자신의 서브 디렉토리로 강제 지정
2. **이데아 규칙** — "자신의 디렉토리 외부 파일 수정 금지"를 시스템 프롬프트에 명시

```python
# subprocess 실행 시 WorkDir 고정
subprocess.run(
    ["claude", "--print", "--dangerously-skip-permissions", prompt],
    cwd=str(agent.work_dir),   # ← 자신의 서브 디렉토리로 강제
)
```

## 2.3 Phase 2 — --allowedTools 도구 제한

안정화 이후 `--allowedTools` 플래그로 에이전트가 사용할 수 있는 도구를 명시적으로 제한합니다.

```bash
# 파일 읽기/쓰기만 허용, 쉘 실행 금지
claude --allowedTools "Read,Write,Edit" --print "..."

# 특정 경로만 허용 (지원 시)
claude --allowedTools "Read(./backend/**),Write(./backend/**)" --print "..."
```

Consumer 에이전트(코드 리뷰어, 문서 작성자)는 Write 도구를 자신의 디렉토리로만 제한합니다.

## 2.4 Phase 4 — Docker 컨테이너 격리 (최종)

최종 단계에서 각 에이전트는 독립된 Docker 컨테이너로 실행됩니다.
subprocess 방식과 달리 OS 레벨에서 완전히 격리됩니다.

```
현재 (subprocess)                  최종 (Docker)
─────────────────────────────────────────────────────
에이전트가 이데아 무시하면          컨테이너 경계로
→ ../다른팀원/ 건드릴 수 있음       → 물리적으로 불가능
→ 시스템 파일 건드릴 수 있음        → 마운트된 경로만 접근
→ 네트워크 마음대로 사용            → 네트워크 격리
```

**Docker 실행 구조:**

```bash
docker run \
  --rm \
  -v ./backend:/workspace \       # 자기 서브 디렉토리만 마운트
  -v ./.orchestra:/orchestra:ro \ # 설정 읽기 전용
  --network none \                 # 네트워크 완전 차단 (또는 필요한 것만 허용)
  --memory 2g \                    # 메모리 제한
  --cpus 1.0 \                     # CPU 제한
  claude-agent:latest \
  claude --print "{prompt}"
```

**팀원 컨테이너 간 통신:**
- 컨테이너끼리 직접 통신 없음
- **파일 시스템(마운트 볼륨)만으로 통신**
- 팀장만 전체 디렉토리 마운트 보유
- Consumer 에이전트는 Producer 디렉토리를 읽기 전용으로 추가 마운트

```
Lead Container          Backend Container
  /workspace/    ←→      /workspace/         (backend/ 만 마운트)
    backend/               .agent-status
    frontend/
    db/
    .orchestra/
```

**코드 변경 범위 (subprocess → Docker):**

Go 코드 기준으로 `agent.go`의 `Start()` 함수 내부만 교체하면 됩니다.
나머지 오케스트레이션 로직은 그대로 유지됩니다.

```go
// Phase 1~3: subprocess
cmd := exec.Command("claude", "--print", prompt)
cmd.Dir = agent.WorkDir

// Phase 4: Docker (이 부분만 교체)
cmd := exec.Command("docker", "run",
    "--rm",
    "-v", agent.WorkDir+":/workspace",
    "--network", "none",
    "--memory", "2g",
    "claude-agent:latest",
    "claude", "--print", prompt,
)
```

---

# 3. 시스템 아키텍처

## 3.1 전체 구조도

```
┌─────────────────────────────────────────────────────┐
│                   GUI Layer (Wails)   ← Phase 3~     │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────┐  │
│  │  Node Editor  │  │ Idea Editor  │  │  Log View │  │
│  │ (React Flow)  │  │  (textarea)  │  │ (stream)  │  │
│  └──────┬────────┘  └──────┬───────┘  └─────┬─────┘  │
└─────────┼──────────────────┼────────────────┼────────┘
          │                  │                │
┌─────────▼──────────────────▼────────────────▼────────┐
│            Orchestration Engine                       │
│         (Python MVP → Go 최종)                        │
│  ┌─────────────────────────────────────────────────┐  │
│  │              AgentManager                        │  │
│  │  ┌────────────┐    ┌────────────────────────┐   │  │
│  │  │ LeadAgent  │───▶│  TaskRouter / Scheduler │   │  │
│  │  └────────────┘    └──────────┬─────────────┘   │  │
│  │                               │                  │  │
│  │  ┌────────────────────────────▼───────────────┐  │  │
│  │  │           SubAgent Pool                     │  │  │
│  │  │  [Backend] [Frontend] [DB] [Doc] [Reviewer] │  │  │
│  │  └────────────────────────────────────────────┘  │  │
│  └─────────────────────────────────────────────────┘  │
└───────────────────────────┬───────────────────────────┘
                            │
         ┌──────────────────┴──────────────────┐
         │                                     │
┌────────▼────────┐                  ┌─────────▼───────┐
│  Phase 1~3      │                  │  Phase 4~5       │
│  subprocess     │                  │  Docker          │
│  + PTY (Go)     │                  │  컨테이너         │
└─────────────────┘                  └─────────────────┘
```

## 3.2 워크스페이스 디렉토리 구조

팀장은 메인 디렉토리 전체를 playground로 사용하며,
각 팀원은 자신의 서브 디렉토리만 접근합니다.
Consumer 에이전트(코드 리뷰어, 문서 작성자)는 다른 서브 디렉토리를 읽기 전용으로 참조합니다.

```
MainWorkspace/                    ← 팀장(Lead) playground
│
├── .orchestra/                   ← Orchestra 내부 설정
│   ├── config.yaml               ← 프로젝트 설정 (등록된 에이전트 목록)
│   ├── ideas/                    ← 에이전트별 이데아 (사용자 편집 가능)
│   │   ├── lead.yaml
│   │   ├── backend.yaml
│   │   ├── frontend.yaml
│   │   └── reviewer.yaml
│   ├── tasks/                    ← 태스크 그래프 & 실행 상태
│   └── locks/                    ← 파일 락 레지스트리 (Go 단계~)
│
├── backend/                      ← Backend 팀원 playground
│   ├── src/
│   └── .agent-status             ← 작업 상태 신호 파일 (IDLE/RUNNING/DONE/ERROR)
│
├── frontend/                     ← Frontend 팀원 playground
│   ├── components/
│   └── .agent-status
│
├── db/                           ← DB 팀원 playground
│   ├── migrations/
│   └── .agent-status
│
├── doc-writer/                   ← 문서 작성자 (Consumer — 읽기 전용 참조)
│   ├── docs/
│   └── .agent-status             ← backend/, frontend/ 읽기 참조
│
└── code-reviewer/                ← 코드 리뷰어 (Consumer — 읽기 전용 참조)
    ├── reviews/
    └── .agent-status             ← 모든 Producer 디렉토리 읽기 참조
```

## 3.3 에이전트 유형 분류

| 유형 | 예시 역할 | 워크스페이스 권한 | 크로스 참조 | Docker 마운트 |
|------|-----------|------------------|------------|--------------|
| Producer | Backend, Frontend, DB | 자신의 서브 dir 읽기/쓰기 | 없음 | 자기 dir만 rw |
| Consumer | Code Reviewer, Doc Writer | 자신의 서브 dir 읽기/쓰기 | 다른 서브 dirs 읽기 전용 | 자기 dir rw + 타 dirs ro |
| Lead | 팀장 (Orchestrator) | 메인 dir 전체 읽기/쓰기 | 모든 dirs | 전체 rw |

---

# 4. 핵심 컴포넌트 설계

## 4.1 파일 구성 (Python MVP)

```
orchestra/
├── main.py           ← CLI 진입점 (init / run / idea / status)
├── lead_agent.py     ← 팀장 AI (태스크 분해 & 배분 & 결과 취합)
├── agent.py          ← 에이전트 기반 클래스 (subprocess 래핑)
├── workspace.py      ← 워크스페이스 & 이데아 YAML 관리
└── requirements.txt  ← pyyaml
```

## 4.2 AgentConfig & Agent (Python)

```python
@dataclass
class AgentConfig:
    agent_id:   str
    role:       str
    idea:       str          # 시스템 프롬프트 (이데아)
    work_dir:   Path
    read_refs:  list[Path]   # Consumer 전용 읽기 전용 참조 경로

class Agent:
    def run(self, instruction: str) -> str:
        """blocking 실행 — 완료까지 대기 후 결과 반환"""

    def run_async(self, instruction: str) -> threading.Thread:
        """비동기 실행 — 스레드 반환"""

    def wait_until_done(self, timeout: float) -> bool:
        """완료 폴링 대기"""
```

## 4.3 이데아(Idea) 시스템

이데아는 각 에이전트의 역할, 전문성, 행동 방침을 정의하는 시스템 프롬프트입니다.
팀장 AI가 역할에 맞는 기본 이데아를 자동 생성하며, 사용자가 자유롭게 편집할 수 있습니다.

> ✏️ **편집 방법**: `python main.py idea --project ./my-project`

```yaml
# .orchestra/ideas/backend.yaml
role: backend
idea: |
  당신은 백엔드 개발 전문가입니다.
  담당 디렉토리: ./backend/

  전문 영역:
  - RESTful API 설계 및 구현
  - 인증/인가 시스템 (JWT, OAuth2)
  - 성능 최적화 및 캐싱 전략

  작업 규칙:
  - 자신의 디렉토리(./backend/) 외부 파일은 절대 수정하지 않습니다.
  - 작업 완료 시 .agent-status 파일에 DONE을 기록합니다.
  - 다른 팀원과의 인터페이스는 주석으로 명확히 명세합니다.
```

## 4.4 팀장 AI의 태스크 분배 파이프라인

```
User Input
    │
    ▼
┌─────────────────────────────────────┐
│  Lead Agent (Claude Code)            │
│  1. 요구사항 분석                     │
│  2. 태스크 분해 (JSON 출력 강제)      │
│  3. 의존성 파악 (parallel 플래그)     │
│  4. 팀원 역할 매핑                   │
│  5. 실행 순서 결정                   │
└──────────────┬──────────────────────┘
               │
               ▼  TaskGraph (JSON)
    ┌──────────────────────┐
    │    TaskScheduler      │
    │  parallel=true  → 동시 실행
    │  parallel=false → 직전 배치 완료 후
    └──────────┬───────────┘
               │
    ┌──────────┴──────────┐
    ▼          ▼          ▼
 [Backend]  [Frontend]  [DB]      ← parallel=true, 동시 실행
    │
    ▼ 완료 후 (parallel=false)
 [Reviewer] [DocWriter]           ← Producer 완료 후 순차 실행
    │
    ▼ 모두 완료
 Lead Agent → 결과 취합 → 최종 보고서
```

## 4.5 태스크 분해 JSON 포맷

팀장 AI는 반드시 아래 형식의 JSON으로만 응답합니다.

```json
[
  {
    "agent_id": "backend",
    "instruction": "사용자 인증 API (POST /auth/login, POST /auth/logout)를 JWT 기반으로 구현하세요.",
    "parallel": true
  },
  {
    "agent_id": "frontend",
    "instruction": "로그인 폼 컴포넌트를 React로 구현하세요. 백엔드 API: POST /auth/login",
    "parallel": true
  },
  {
    "agent_id": "db",
    "instruction": "users 테이블 스키마와 마이그레이션 스크립트를 작성하세요.",
    "parallel": true
  },
  {
    "agent_id": "reviewer",
    "instruction": "backend/, frontend/, db/ 코드를 리뷰하고 개선사항을 ./reviewer/review.md에 작성하세요.",
    "parallel": false
  }
]
```

## 4.6 AgentNode 구조체 (Go — Phase 3~)

```go
type AgentNode struct {
    ID          string
    Role        string
    RoleType    AgentRoleType    // Producer | Consumer | Lead
    WorkDir     string
    Idea        string

    // Phase 1~3: subprocess
    Process     *os.Process
    PTY         *os.File

    // Phase 4~: Docker
    ContainerID string

    // State
    Status      AgentStatus      // Idle | Running | Waiting | Done | Error
    CurrentTask *Task
    OutputChan  chan string
    DoneChan    chan struct{}

    ReadRefs    []string         // Consumer 전용
    mu          sync.RWMutex
}
```

---

# 5. 워크스페이스 충돌 방지

## 5.1 3단계 방어 전략

| 단계 | 방법 | 적용 시점 |
|------|------|-----------|
| 1차 | 디렉토리 격리 (WorkDir 고정) | Phase 1~ (항상) |
| 2차 | 이데아 규칙 (외부 수정 금지 명시) | Phase 1~ (항상) |
| 3차 | 파일 락 레지스트리 | Phase 3~ (Go 단계) |
| 최종 | Docker 컨테이너 경계 | Phase 4~ |

## 5.2 .agent-status 완료 감지

각 에이전트는 작업 완료 시 자신의 디렉토리에 `.agent-status` 파일을 씁니다.
팀장은 이 파일을 감시하여 다음 태스크 실행을 결정합니다.

```
IDLE     → 대기 중
RUNNING  → 작업 중
DONE     → 완료
ERROR    → 오류 발생
```

**Python MVP:** polling 방식으로 1초마다 확인
**Go 단계:** fsnotify로 이벤트 기반 감지

## 5.3 크로스 참조 에이전트 (Consumer)

코드 리뷰어, 문서 작성자는 Producer들이 모두 완료된 후 실행됩니다.
팀장이 시작 시점에 참조 경로를 이데아에 동적으로 주입합니다.

```python
# Consumer 시작 시 팀장이 참조 경로 자동 주입
enriched_idea = agent.idea + """

[읽기 전용 참조 디렉토리 — 수정 금지]
- ./backend/
- ./frontend/
- ./db/
"""
```

---

# 6. GUI 설계 (Phase 3~ / Wails v2 + React Flow)

## 6.1 주요 화면 구성

| 화면 | 설명 | 주요 기능 |
|------|------|-----------|
| Node Editor | 에이전트 노드 시각화 | 노드 추가/삭제, 연결선, 드래그 |
| Idea Editor | 이데아 편집 패널 | 노드 클릭 시 열리는 텍스트 편집기 |
| Task Monitor | 태스크 실행 현황 | 태스크 그래프, 진행률, 상태 |
| Log Viewer | 실시간 에이전트 출력 | 에이전트별 로그 스트리밍 |
| Workspace Explorer | 디렉토리 구조 탐색 | 파일 트리, 읽기 전용 구분 표시 |

## 6.2 Wails 백엔드 바인딩 (Go)

```go
// Go 함수를 React에서 직접 호출
// window.go.App.AddAgent("backend", "producer")

func (a *App) AddAgent(role string, roleType string) (string, error)
func (a *App) UpdateIdea(agentID string, idea string) error
func (a *App) SubmitUserInput(input string) error

// 실시간 로그 스트리밍
func (a *App) streamLogs(agentID string, ch chan string) {
    for log := range ch {
        runtime.EventsEmit(a.ctx, "log:"+agentID, log)
    }
}
```

---

# 7. 개발 단계 로드맵

## Phase 1 — Python MVP (현재)

> 🎯 **목표**: 오케스트레이션 핵심 로직 검증. GUI 없음.

**검증 항목:**
- Claude Code subprocess 실행 & I/O 제어 가능한가?
- 팀장이 팀원에게 지시를 넣고 결과를 받을 수 있는가?
- 디렉토리 격리가 실제로 작동하는가?
- 병렬/순차 실행 조율이 안정적인가?

**범위:**
```
✅ 포함                         ❌ 제외
─────────────────────────────────────────
팀장 1명 + 팀원 N명             GUI
subprocess Claude Code 실행     노드 에디터
디렉토리 격리                   파일 락 레지스트리
팀장 → 팀원 태스크 분배         Docker
.agent-status 완료 감지         동적 팀원 추가 UI
이데아 YAML 편집 (CLI)
터미널 출력
```

**사용법:**
```bash
pip install pyyaml
python main.py -p ./my-project init --roles backend,frontend,db,reviewer
python main.py -p ./my-project run
python main.py -p ./my-project idea    # 이데아 편집
python main.py -p ./my-project status  # 상태 확인
```

## Phase 2 — 권한 제어 강화

> 🎯 **목표**: `--allowedTools`로 에이전트 도구 접근 제한

1. Claude Code `--allowedTools` 플래그 적용
2. Producer / Consumer 별 허용 도구 분리
3. 파일 접근 경로 제한 테스트

## Phase 3 — Go + Wails GUI

> 🎯 **목표**: 검증된 Python 로직을 Go로 포팅 + GUI 추가

1. Go 프로젝트 초기화 (Wails v2 scaffolding)
2. Python 로직 → Go 포팅 (AgentNode, LeadAgent, Workspace)
3. PTY subprocess 래핑 (`creack/pty`)
4. React Flow 노드 에디터
5. 이데아 편집 사이드패널
6. 실시간 로그 스트리밍 (Wails EventsEmit)
7. fsnotify 기반 완료 감지 전환

## Phase 4 — Docker 컨테이너 격리

> 🎯 **목표**: subprocess → Docker 교체로 OS 레벨 샌드박스 구현

1. `claude-agent` Docker 이미지 작성
2. `agent.go`의 `Start()` 함수 내부만 Docker 실행으로 교체
3. 볼륨 마운트 전략 (rw / ro) 적용
4. 네트워크 격리 (`--network none` 또는 커스텀 네트워크)
5. 리소스 제한 (`--memory`, `--cpus`)
6. 컨테이너 생명주기 관리

## Phase 5 — Docker Compose 통합

> 🎯 **목표**: 전체 스택을 Docker Compose로 선언적 관리

```yaml
# docker-compose.yml 예시
services:
  lead:
    image: claude-agent:latest
    volumes:
      - ./workspace:/workspace
  backend:
    image: claude-agent:latest
    volumes:
      - ./workspace/backend:/workspace
    network_mode: none
  frontend:
    image: claude-agent:latest
    volumes:
      - ./workspace/frontend:/workspace
    network_mode: none
```

---

# 8. 주요 도전 과제 & 해결 전략

| 도전 과제 | 난이도 | 해결 전략 |
|-----------|--------|-----------|
| Claude Code stdout 파싱 | 높음 | `--output-format stream-json` 옵션 활용, PTY로 대화형 처리 |
| 작업 완료 감지 신뢰성 | 중간 | `.agent-status` 파일 + fsnotify (Go), polling (Python), timeout 폴백 |
| 팀원 간 파일 충돌 | 중간 | 디렉토리 격리 → 이데아 규칙 → 파일 락 → Docker 경계 (단계별 강화) |
| 팀장 AI 태스크 분해 품질 | 높음 | 구조화된 프롬프트 + JSON 출력 강제 + 유효성 검증 + 폴백 로직 |
| Windows PTY 지원 | 중간 | `github.com/creack/pty` Windows 지원, ConPTY API 폴백 |
| Docker 이미지 Claude Code 설치 | 중간 | Dockerfile에 Claude Code CLI 설치 스크립트 포함 |
| 에이전트 간 인터페이스 계약 | 중간 | `.orchestra/contracts/`에 API 명세 공유, 팀장이 조율 |

---

# 9. 기술 의존성 목록

**Python MVP:**

| 패키지 | 용도 |
|--------|------|
| `pyyaml` | 이데아 YAML 저장/로드 |

**Go (Phase 3~):**

| 패키지 | 용도 | 라이선스 |
|--------|------|----------|
| `github.com/wailsapp/wails/v2` | GUI 프레임워크 | MIT |
| `github.com/creack/pty` | PTY 지원 | MIT |
| `github.com/fsnotify/fsnotify` | 파일 시스템 감시 | BSD-3 |
| `github.com/google/uuid` | 에이전트 고유 ID | BSD-3 |
| `gopkg.in/yaml.v3` | 이데아 YAML | Apache-2.0 |
| `reactflow` | 노드 에디터 UI | MIT |
| `zustand` | React 상태 관리 | MIT |

---

# 10. 미결 사항 & 추후 논의

- Claude Code CLI의 `--output-format stream-json` 정식 지원 여부 확인
- `--allowedTools` 플래그의 경로 제한 문법 확인 (Phase 2 진입 전)
- Docker 이미지 내 Claude Code 인증 처리 방법 (API 키 주입 전략)
- 에이전트 간 계약(interface contract) 포맷 확정 (JSON Schema vs OpenAPI)
- 멀티 세션 저장/복원 전략 — 프로젝트 단위 vs 세션 단위
- 에이전트 비용 추적 — Claude Code 사용량 모니터링
- Docker Compose vs Kubernetes (규모에 따라)
- Windows에서 Docker 성능 이슈 사전 검토 (WSL2 기반)
