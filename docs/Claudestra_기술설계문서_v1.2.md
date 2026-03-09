# CLAUDESTRA
## Claude Code Multi-Agent Orchestration System
### 기술 설계 문서 v1.2

| 항목 | 내용 |
|------|------|
| 프로젝트명 | Claudestra |
| MVP 언어 | Python |
| 최종 언어 | Go + React (Wails v2) |
| 대상 플랫폼 | Linux / Windows |
| AI 엔진 | Claude Code CLI |
| 에이전트 격리 | subprocess (MVP) → Docker 컨테이너 (최종) |
| 문서 버전 | 1.2.0 |

---

# 1. 프로젝트 개요

## 1.1 목표

Claudestra는 여러 Claude Code 인스턴스를 오케스트레이션하여 복잡한 소프트웨어 개발 작업을 자동화하는 멀티 에이전트 시스템입니다.

사용자는 팀장(Lead Agent)과 다양한 역할의 팀원(Sub-Agent)을 구성하고, 팀장 AI가 사용자의 요구사항을 분석하여 각 팀원에게 최적화된 태스크를 배분합니다.

> **핵심 컨셉**: 팀장 AI = 오케스트레이터 / 팀원 AI = 전문 실행자.
> 각자 격리된 워크스페이스에서 작업하며 팀장이 전체를 조율합니다.

## 1.2 개발 전략 — 단계별 접근

이 프로젝트의 가장 큰 불확실성은 **"Claude Code subprocess를 실제로 안정적으로 제어할 수 있냐"** 입니다.
GUI나 언어 포팅은 그 이후 문제입니다. 따라서 검증 우선 전략을 취합니다.

```
Phase 1  Python MVP          오케스트레이션 핵심 로직 검증        ← 현재
Phase 2  권한 제어 강화       --allowedTools 도구 제한 추가
Phase 3  Go + Wails GUI       검증된 로직을 Go로 포팅 + GUI
Phase 4  Docker 격리          컨테이너 기반 진짜 샌드박스
Phase 5  Docker Compose       전체 스택 통합 관리
```

## 1.3 주요 특징

- Claude Code CLI를 subprocess로 래핑하여 각 에이전트 독립 실행
- **Plan → Approve → Execute** 워크플로우 — 팀장이 계획 수립 후 사용자 승인을 거쳐 실행
- **인터페이스 계약서(Contract)** 시스템 — 팀원 간 API/DB/네이밍 스펙 통일
- 역할별 **이데아(Idea)** 시스템 — 사용자 편집 가능한 시스템 프롬프트
- **파일 락 레지스트리** — 에이전트 간 워크스페이스 충돌 방지
- 디렉토리 격리 + 이데아 규칙으로 다단계 방어
- 코드 리뷰어, 문서 작성자 등 크로스 참조(Consumer) 에이전트 지원
- 비개발 입력(인사, 잡담) 자동 감지 — 팀장이 직접 응답하여 불필요한 API 호출 방지
- React Flow 기반 노드 에디터로 팀 구성 시각화 (Phase 3~)
- **최종적으로 Docker 컨테이너 기반 OS 레벨 격리** 적용

## 1.4 기술 스택 요약

| 레이어 | MVP (Python) | 최종 (Go) |
|--------|-------------|-----------|
| 언어 | Python 3.11+ | Go 1.22+ |
| GUI | 없음 (CLI) | Wails v2 + React Flow |
| 에이전트 실행 | subprocess + `--dangerously-skip-permissions` | Docker 컨테이너 |
| 상태 관리 | threading + `.agent-status` 파일 | goroutine + channel |
| 파일 감시 | polling / `.agent-status` | fsnotify |
| 파일 잠금 | FileLockRegistry (threading.Lock + JSON) | FileLockRegistry (sync.RWMutex) |
| 계약서 | YAML (.orchestra/contracts/) | YAML |
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
+ 파일 락 레지스트리
+ 인터페이스 계약서
```

## 2.2 Phase 1 — skip-permissions + 다단계 방어 (현재)

MVP 단계에서는 `--dangerously-skip-permissions` 플래그로 permission 프롬프트를 건너뜁니다.
대신 네 가지로 리스크를 관리합니다.

**왜 permission 릴레이 방식은 안 쓰나?**

permission 릴레이란 에이전트의 permission 프롬프트를 팀장이 캡처해서 사용자에게 전달하고, 응답을 다시 주입하는 방식입니다.
팀원이 5명이면 동시다발로 permission이 터지고, UX도 엉망이 되며, 구현 복잡도만 높아집니다. 실용적이지 않습니다.

**대신 이 네 가지로 방어합니다:**

1. **WorkDir 격리** — 각 에이전트의 작업 디렉토리를 자신의 서브 디렉토리로 강제 지정
2. **이데아 규칙** — "자신의 디렉토리 외부 파일 수정 금지"를 시스템 프롬프트에 명시
3. **파일 락 레지스트리** — 에이전트 실행 시 디렉토리 잠금 획득, 충돌 시 실행 차단
4. **인터페이스 계약서** — 팀장이 공통 스펙을 생성하여 모든 에이전트에 주입

```python
# 1. WorkDir 격리
subprocess.run(
    ["claude", "--print", "--dangerously-skip-permissions", prompt],
    cwd=str(agent.work_dir),   # ← 자신의 서브 디렉토리로 강제
)

# 2. 이데아 규칙 (시스템 프롬프트에 포함)
# "자신의 디렉토리(./backend/) 외부 파일은 절대 수정하지 않습니다."

# 3. 파일 락 (실행 전 자동 획득, 완료 후 자동 해제)
lock_registry.acquire(str(agent.work_dir), agent.agent_id)
# ... 작업 실행 ...
lock_registry.release_all(agent.agent_id)  # finally 블록에서 해제

# 4. 계약서 (프롬프트에 주입)
# "[인터페이스 계약서 — 반드시 이 명세를 따르세요]"
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
  -v ./.orchestra:/orchestra:ro \ # 설정 읽기 전용 (계약서 포함)
  --network none \                 # 네트워크 완전 차단
  --memory 2g \                    # 메모리 제한
  --cpus 1.0 \                     # CPU 제한
  claude-agent:latest \
  claude --print "{prompt}"
```

**코드 변경 범위 (subprocess → Docker):**

Go 코드 기준으로 `agent.go`의 `Start()` 함수 내부만 교체하면 됩니다.
나머지 오케스트레이션 로직(계획 수립, 계약서, 잠금 등)은 그대로 유지됩니다.

```go
// Phase 1~3: subprocess
cmd := exec.Command("claude", "--print", prompt)
cmd.Dir = agent.WorkDir

// Phase 4: Docker (이 부분만 교체)
cmd := exec.Command("docker", "run",
    "--rm",
    "-v", agent.WorkDir+":/workspace",
    "-v", agent.ContractsDir+":/orchestra/contracts:ro",
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
│  │  │ LeadAgent  │───▶│  Plan-Approve-Execute   │   │  │
│  │  │            │    │  + Contract Generator   │   │  │
│  │  └────────────┘    └──────────┬─────────────┘   │  │
│  │                               │                  │  │
│  │  ┌────────────────────────────▼───────────────┐  │  │
│  │  │           SubAgent Pool                     │  │  │
│  │  │  [Backend] [Frontend] [DB] [Doc] [Reviewer] │  │  │
│  │  └────────────────────────────────────────────┘  │  │
│  │                                                   │  │
│  │  ┌─────────────────────────────────────────────┐  │  │
│  │  │  FileLockRegistry    ContractStore           │  │  │
│  │  └─────────────────────────────────────────────┘  │  │
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

## 3.2 핵심 워크플로우: Plan → Approve → Execute

v1.2에서 가장 중요한 변경사항입니다. 기존의 "즉시 실행" 방식에서 계획-승인-실행 방식으로 전환했습니다.

```
사용자 입력: "학원 앱 만들어줘"
    │
    ▼
┌─────────────────────────────────────┐
│  1. 계획 수립 (_decompose)           │
│     팀장 AI가 단계별 실행 계획 생성   │
│     → 단계(step) 기반 태스크 분해     │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│  2. 계약서 생성 (_generate_contract) │
│     기술스택, API 명세, DB 스키마,    │
│     네이밍 규칙 등 공통 스펙 작성     │
│     → .orchestra/contracts/ 에 저장  │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│  3. 사용자에게 계획+계약서 표시      │
│     "진행할까요?"                    │
│     y: 전체 실행                     │
│     s: 단계별 확인 (단계마다 y/n)    │
│     n: 취소                          │
└──────────────┬──────────────────────┘
               │ 승인
               ▼
┌─────────────────────────────────────┐
│  4. 계약서를 모든 에이전트에 주입     │
│     agent.config.contract = contract │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│  5. 단계별 실행                      │
│     1단계: [DB] [Backend] 병렬 실행  │
│        ↓ 완료                        │
│     2단계: [Frontend] 실행           │
│        ↓ 완료                        │
│     3단계: [Reviewer] 실행           │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│  6. 최종 보고서 작성 (_summarize)    │
└─────────────────────────────────────┘
```

**비개발 입력 처리:**

"안녕?", "고마워" 같은 비개발 입력은 팀장이 빈 태스크 배열(`[]`)을 반환하고 직접 응답합니다.
에이전트를 동원하지 않으므로 API 호출 1회로 처리됩니다 (기존 6회 → 1회).

## 3.3 워크스페이스 디렉토리 구조

```
MainWorkspace/                    ← 팀장(Lead) playground
│
├── .orchestra/                   ← Claudestra 내부 설정
│   ├── config.yaml               ← 프로젝트 설정 (등록된 에이전트 목록)
│   ├── ideas/                    ← 에이전트별 이데아 (사용자 편집 가능)
│   │   ├── backend.yaml
│   │   ├── frontend.yaml
│   │   └── reviewer.yaml
│   ├── contracts/                ← 인터페이스 계약서
│   │   └── contract.yaml         ← 현재 활성 계약서
│   └── locks/                    ← 파일 락 레지스트리
│       └── registry.json         ← 현재 잠금 상태
│
├── backend/                      ← Backend 팀원 playground
│   ├── src/
│   └── .agent-status
│
├── frontend/                     ← Frontend 팀원 playground
│   ├── components/
│   └── .agent-status
│
├── db/                           ← DB 팀원 playground
│   ├── migrations/
│   └── .agent-status
│
├── doc-writer/                   ← 문서 작성자 (Consumer)
│   ├── docs/
│   └── .agent-status
│
└── code-reviewer/                ← 코드 리뷰어 (Consumer)
    ├── reviews/
    └── .agent-status
```

## 3.4 에이전트 유형 분류

| 유형 | 예시 역할 | 워크스페이스 권한 | 크로스 참조 | Docker 마운트 |
|------|-----------|------------------|------------|--------------|
| Producer | Backend, Frontend, DB | 자신의 서브 dir 읽기/쓰기 | 없음 | 자기 dir만 rw |
| Consumer | Code Reviewer, Doc Writer | 자신의 서브 dir 읽기/쓰기 | 다른 서브 dirs 읽기 전용 | 자기 dir rw + 타 dirs ro |
| Lead | 팀장 (Orchestrator) | 메인 dir 전체 읽기/쓰기 | 모든 dirs | 전체 rw |

---

# 4. 핵심 컴포넌트 설계

## 4.1 파일 구성 (Python MVP)

```
Claudestra/
├── src/
│   ├── main.py           ← CLI 진입점 (init / run / idea / status)
│   ├── lead_agent.py     ← 팀장 AI (계획 수립, 계약서 생성, 실행 조율, 보고)
│   ├── agent.py          ← 에이전트 기반 클래스 (subprocess 래핑 + 잠금)
│   ├── workspace.py      ← 워크스페이스, 이데아, 계약서 관리
│   └── file_lock.py      ← 파일 락 레지스트리
├── docs/                 ← 설계 문서
├── Dockerfile            ← Docker 테스트 환경
├── run.sh                ← Docker 빌드 & 실행 스크립트
└── .gitignore
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
    contract:   str = ""     # 인터페이스 계약서 (팀장이 주입)

class Agent:
    def __init__(self, config: AgentConfig, lock_registry=None):
        """lock_registry가 주어지면 실행 시 자동 잠금"""

    def run(self, instruction: str) -> str:
        """blocking 실행 — 잠금 획득 → 작업 → 잠금 해제"""

    def run_async(self, instruction: str) -> threading.Thread:
        """비동기 실행 — 스레드 반환"""

    def _build_prompt(self, instruction: str) -> str:
        """이데아 + 작업 원칙 + 계약서 + 크로스 참조 + 지시를 결합"""
```

**에이전트 프롬프트 구조:**

```
[이데아] 당신은 백엔드 개발 전문가입니다...

[작업 원칙]
- 간결하게 핵심만 작성하세요.
- 핵심 구조, 주요 파일, 중요 로직만 구현하세요.

[인터페이스 계약서 — 반드시 이 명세를 따르세요]
tech_stack:
  backend: Express.js
  db: PostgreSQL
api_endpoints:
  - POST /api/auth/login
  ...

[읽기 전용 참조 디렉토리 — 수정 금지]   ← Consumer만
  - ./backend/
  - ./frontend/

[지시] 사용자 인증 API를 구현하세요...
```

## 4.3 이데아(Idea) 시스템

이데아는 각 에이전트의 역할, 전문성, 행동 방침을 정의하는 시스템 프롬프트입니다.
팀장 AI가 역할에 맞는 기본 이데아를 자동 생성하며, 사용자가 자유롭게 편집할 수 있습니다.

> **편집 방법**: `python main.py idea -p ./my-project`

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
  - 인터페이스 계약서가 제공되면 반드시 그 명세를 따릅니다.
```

## 4.4 인터페이스 계약서(Contract) 시스템

v1.2에서 새로 도입된 핵심 기능입니다. 팀원 간 인터페이스 불일치를 방지합니다.

**문제:** 격리된 에이전트들이 각자 독립적으로 네이밍, API 스펙, DB 엔진을 결정하면
백엔드는 `teacher`, 프론트는 `instructor`를 쓰는 등 통합 불가능한 결과물이 나옴.

**해결:** 팀장이 계획 수립 직후, 실행 전에 공통 계약서를 생성하여 모든 에이전트에 주입.

```yaml
# .orchestra/contracts/contract.yaml (팀장이 자동 생성)
tech_stack:
  language: TypeScript
  backend: Express.js
  frontend: React Native
  database: PostgreSQL

naming_conventions:
  fields: snake_case
  api_paths: kebab-case
  identifiers: English only

api_endpoints:
  - method: POST
    path: /api/auth/login
    request: { email: string, password: string }
    response: { access_token: string, refresh_token: string }
  - method: GET
    path: /api/students
    response: { students: Student[] }

db_schema:
  students:
    - id: serial PRIMARY KEY
    - name: varchar(100)
    - parent_id: integer REFERENCES parents(id)

shared_types:
  Student: { id, name, grade, parent_id, created_at }
  Teacher: { id, name, subject, phone }
```

**계약서 생명주기:**

```
1. 팀장이 _generate_contract() 로 Claude를 호출하여 YAML 생성
2. 사용자에게 계획과 함께 계약서 표시
3. 사용자가 승인하면 .orchestra/contracts/contract.yaml 에 저장
4. 모든 에이전트의 config.contract 에 주입
5. 에이전트 프롬프트에 "[인터페이스 계약서 — 반드시 이 명세를 따르세요]" 로 포함
```

## 4.5 파일 락 레지스트리

에이전트 간 워크스페이스 충돌을 방지하는 2차 방어 메커니즘입니다.

```python
class FileLockRegistry:
    """Thread-safe 파일 잠금 레지스트리"""

    def acquire(self, file_path, agent_id):
        """잠금 획득. 다른 에이전트가 보유 중이면 LockConflictError"""

    def release_all(self, agent_id):
        """에이전트의 모든 잠금 해제 (작업 완료 후)"""

    def list_locks(self):
        """현재 잠금 상태 조회 (status 명령에서 사용)"""
```

**동작 흐름:**

```
Agent.run() 시작
  → lock_registry.acquire(work_dir, agent_id)  # 잠금 획득
  → 🔒 잠금 획득 로그 출력
  → subprocess 실행 (claude --print)
  → 작업 완료/실패/타임아웃
  → lock_registry.release_all(agent_id)  # finally 블록에서 해제
  → 🔓 잠금 해제 로그 출력
```

잠금 상태는 `.orchestra/locks/registry.json`에 저장되어 `status` 명령으로 확인 가능합니다.

## 4.6 팀장 AI의 태스크 분해 포맷

팀장 AI는 단계(step) 기반 JSON으로 응답합니다.
같은 단계의 태스크는 병렬 실행, 단계 간에는 순차 실행됩니다.

```json
[
  {
    "step": 1,
    "title": "DB 스키마 설계",
    "tasks": [
      {"agent_id": "db", "instruction": "학생, 학부모, 수업 테이블 설계"}
    ]
  },
  {
    "step": 2,
    "title": "백엔드 + 프론트엔드 구현",
    "tasks": [
      {"agent_id": "backend", "instruction": "학생 CRUD API 구현"},
      {"agent_id": "frontend", "instruction": "로그인, 학생 목록 화면 구현"}
    ]
  },
  {
    "step": 3,
    "title": "코드 리뷰",
    "tasks": [
      {"agent_id": "reviewer", "instruction": "전체 아키텍처 리뷰"}
    ]
  }
]
```

## 4.7 AgentNode 구조체 (Go — Phase 3~)

```go
type AgentNode struct {
    ID          string
    Role        string
    RoleType    AgentRoleType    // Producer | Consumer | Lead
    WorkDir     string
    Idea        string
    Contract    string           // 인터페이스 계약서

    // Phase 1~3: subprocess
    Process     *os.Process
    PTY         *os.File

    // Phase 4~: Docker
    ContainerID string

    // State
    Status      AgentStatus
    CurrentTask *Task
    OutputChan  chan string
    DoneChan    chan struct{}

    ReadRefs    []string         // Consumer 전용
    mu          sync.RWMutex
}
```

---

# 5. 워크스페이스 충돌 방지

## 5.1 4단계 방어 전략

| 단계 | 방법 | 적용 시점 | 설명 |
|------|------|-----------|------|
| 1차 | 디렉토리 격리 (WorkDir 고정) | Phase 1~ (항상) | 각 에이전트 cwd를 서브 디렉토리로 강제 |
| 2차 | 이데아 규칙 (외부 수정 금지 명시) | Phase 1~ (항상) | 시스템 프롬프트로 행동 제한 |
| 3차 | 파일 락 레지스트리 | Phase 1~ (구현됨) | 동일 디렉토리 동시 접근 차단 |
| 4차 | 인터페이스 계약서 | Phase 1~ (구현됨) | 공통 스펙으로 결과물 호환성 보장 |
| 최종 | Docker 컨테이너 경계 | Phase 4~ | OS 레벨 물리적 격리 |

## 5.2 .agent-status 완료 감지

각 에이전트는 작업 완료 시 자신의 디렉토리에 `.agent-status` 파일을 씁니다.

```
IDLE     → 대기 중
RUNNING  → 작업 중
DONE     → 완료
ERROR    → 오류 발생
```

**Python MVP:** threading + join 방식으로 완료 대기
**Go 단계:** fsnotify로 이벤트 기반 감지

## 5.3 크로스 참조 에이전트 (Consumer)

코드 리뷰어, 문서 작성자는 Producer들이 모두 완료된 후 실행됩니다.
팀장이 시작 시점에 참조 경로를 이데아에 동적으로 주입합니다.

```python
# Consumer 프롬프트에 자동 추가
[읽기 전용 참조 디렉토리 — 수정 금지]
- ./backend/
- ./frontend/
- ./db/
```

---

# 6. GUI 설계 (Phase 3~ / Wails v2 + React Flow)

## 6.1 주요 화면 구성

| 화면 | 설명 | 주요 기능 |
|------|------|-----------|
| Node Editor | 에이전트 노드 시각화 | 노드 추가/삭제, 연결선, 드래그 |
| Idea Editor | 이데아 편집 패널 | 노드 클릭 시 열리는 텍스트 편집기 |
| Contract Viewer | 계약서 편집/조회 | 현재 활성 계약서 표시 및 편집 |
| Task Monitor | 태스크 실행 현황 | 단계별 진행률, 상태 |
| Log Viewer | 실시간 에이전트 출력 | 에이전트별 로그 스트리밍 |
| Workspace Explorer | 디렉토리 구조 탐색 | 파일 트리, 잠금 상태 표시 |

## 6.2 Wails 백엔드 바인딩 (Go)

```go
func (a *App) AddAgent(role string, roleType string) (string, error)
func (a *App) UpdateIdea(agentID string, idea string) error
func (a *App) SubmitUserInput(input string) error
func (a *App) GetContract() (string, error)
func (a *App) UpdateContract(contract string) error
func (a *App) GetLocks() (map[string]string, error)

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

> **목표**: 오케스트레이션 핵심 로직 검증. GUI 없음.

**검증 완료 항목:**
- Claude Code subprocess 실행 & I/O 제어
- 팀장 → 팀원 태스크 분배 & 결과 수집
- 디렉토리 격리 동작 확인
- 병렬/순차(단계별) 실행 조율

**구현 완료:**
```
✅ 팀장 1명 + 팀원 N명 구성
✅ subprocess Claude Code 실행
✅ 디렉토리 격리
✅ Plan → Approve → Execute 워크플로우
✅ 인터페이스 계약서 시스템
✅ 파일 락 레지스트리
✅ 비개발 입력 자동 감지 & 팀장 직접 응답
✅ .agent-status 완료 감지
✅ 이데아 YAML 편집 (CLI)
✅ Docker 테스트 환경 (Dockerfile + run.sh)
```

**사용법:**
```bash
# Docker로 테스트 (호스트에 설치 불필요)
./run.sh

# 컨테이너 내부에서
python main.py init -p /workspace/myproject --roles backend,frontend,db,reviewer
python main.py run -p /workspace/myproject
python main.py idea -p /workspace/myproject
python main.py status -p /workspace/myproject
```

## Phase 2 — 권한 제어 강화

> **목표**: `--allowedTools`로 에이전트 도구 접근 제한

1. Claude Code `--allowedTools` 플래그 적용
2. Producer / Consumer 별 허용 도구 분리
3. 파일 접근 경로 제한 테스트

## Phase 3 — Go + Wails GUI

> **목표**: 검증된 Python 로직을 Go로 포팅 + GUI 추가

1. Go 프로젝트 초기화 (Wails v2 scaffolding)
2. Python 로직 → Go 포팅 (AgentNode, LeadAgent, Workspace, FileLock, Contract)
3. PTY subprocess 래핑 (`creack/pty`)
4. React Flow 노드 에디터
5. 이데아/계약서 편집 사이드패널
6. 실시간 로그 스트리밍 (Wails EventsEmit)
7. fsnotify 기반 완료 감지 전환

## Phase 4 — Docker 컨테이너 격리

> **목표**: subprocess → Docker 교체로 OS 레벨 샌드박스 구현

1. `claude-agent` Docker 이미지 작성
2. `agent.go`의 `Start()` 함수 내부만 Docker 실행으로 교체
3. 볼륨 마운트 전략 (rw / ro) 적용 — 계약서 디렉토리 포함
4. 네트워크 격리 (`--network none` 또는 커스텀 네트워크)
5. 리소스 제한 (`--memory`, `--cpus`)
6. 컨테이너 생명주기 관리

## Phase 5 — Docker Compose 통합

> **목표**: 전체 스택을 Docker Compose로 선언적 관리

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
      - ./workspace/.orchestra/contracts:/contracts:ro
    network_mode: none
  frontend:
    image: claude-agent:latest
    volumes:
      - ./workspace/frontend:/workspace
      - ./workspace/.orchestra/contracts:/contracts:ro
    network_mode: none
```

---

# 8. 주요 도전 과제 & 해결 전략

| 도전 과제 | 난이도 | 해결 전략 | 현재 상태 |
|-----------|--------|-----------|-----------|
| Claude Code stdout 파싱 | 높음 | `--print` 모드 + JSON 출력 강제 | 해결됨 |
| 작업 완료 감지 | 중간 | threading.join (MVP), fsnotify (Go) | 해결됨 |
| 팀원 간 파일 충돌 | 중간 | 디렉토리 격리 + 이데아 규칙 + 파일 락 | 해결됨 |
| 팀원 간 인터페이스 불일치 | 높음 | 인터페이스 계약서 자동 생성 & 주입 | 해결됨 |
| 팀장 AI 태스크 분해 품질 | 높음 | 구조화된 프롬프트 + step 기반 JSON + 검증 + 폴백 | 해결됨 |
| 불필요한 API 호출 | 중간 | 비개발 입력 감지 → 팀장 직접 응답 | 해결됨 |
| 대규모 작업 타임아웃 | 중간 | Plan-Approve-Execute + 간결함 규칙 | 해결됨 |
| Docker 이미지 Claude Code 설치 | 중간 | Dockerfile에 Node.js + Claude Code CLI 설치 | 테스트 환경 구현 |
| Windows PTY 지원 | 중간 | `creack/pty` Windows 지원, ConPTY API 폴백 | Phase 3 |

---

# 9. 기술 의존성 목록

**Python MVP:**

| 패키지 | 용도 |
|--------|------|
| `pyyaml` | 이데아/설정 YAML 저장/로드 |

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
- Docker 이미지 내 Claude Code 인증 처리 방법 (API 키 주입 vs OAuth 마운트)
- 계약서 포맷 고도화 — 현재 자유형 YAML에서 JSON Schema/OpenAPI 기반으로 전환 검토
- 멀티 세션 저장/복원 전략 — 프로젝트 단위 vs 세션 단위
- 에이전트 비용 추적 — Claude Code 사용량 모니터링
- 위상정렬 기반 태스크 그래프 — 현재 step 기반에서 DAG 기반으로 고도화
- Docker Compose vs Kubernetes (규모에 따라)
- Windows에서 Docker 성능 이슈 사전 검토 (WSL2 기반)
- 계약서 사용자 편집 기능 — 자동 생성 후 사용자가 수정하고 재실행
