# CLAUDESTRA
## Claude Code Multi-Agent Orchestration System
### 기술 설계 문서 v2.1

| 항목 | 내용 |
|------|------|
| 프로젝트명 | Claudestra |
| MVP 언어 | Python |
| 최종 언어 | Go + React (Wails v2) |
| 대상 플랫폼 | Linux / Windows |
| AI 엔진 | Claude Code CLI |
| 에이전트 격리 | subprocess (현재) → Docker 컨테이너 (최종) |
| 팀장 도구 방식 | Claudestra CLI 바이너리 (v2.0 핵심) |
| 문서 버전 | 2.1.0 |

---

## v2.1 변경 사항 요약

### 1. GUI UX 개선 — 서브에이전트 분리 표시

v2.0까지는 팀장과 서브에이전트의 메시지가 중앙 로그 패널에 혼재되어 가독성이 떨어졌습니다.

**변경:**
- **중앙 영역**: 팀장(Lead) 메시지만 표시. 서브에이전트 출력 제거
- **사이드바**: 실행 중(RUNNING) 에이전트에 파란색 글로우 효과 추가
- **에이전트 상세 패널**: 서브에이전트의 사고/대화를 여기서 확인. 탭 순서 변경 (받은 지시 → 실행 로그 → 실행 결과)

**구현:**
- `LogEvent` 구조체에 `Agent` 필드 추가 (Go 백엔드 + TS 프론트엔드)
- `log-append` 이벤트에도 agent 정보 전달 → 같은 에이전트의 마지막 메시지에만 append
- LogPanel에서 `logs.filter(l => !l.agent)` 로 팀장 메시지만 필터링

### 2. 텍스트 꼬임 해결

v2.0에서 팀장과 서브에이전트의 스트리밍 텍스트가 동시에 도착하면 append 대상이 잘못되어 텍스트가 섞이는 문제가 있었습니다.

**원인:** `log-append` 이벤트에 agent 식별 정보가 없어서 무조건 마지막 메시지에 append
**해결:** `log-append`도 `LogEvent` 객체로 전송하여 agent별로 정확한 append 대상 매칭

### 3. CLI `wait` 명령 추가 — sleep 폴링 제거

팀장이 `assign --async` 후 완료를 확인하기 위해 `sleep 30 && claudestra status` 패턴을 반복 사용하는 문제가 있었습니다.

**해결:**
- `claudestra wait <agent1> [agent2 ...]` 명령 추가 — RUNNING→DONE/ERROR까지 블로킹 대기
- 팀장 프롬프트에 "sleep 사용 금지, wait 사용 필수" 규칙 추가

### 4. 세션 메모리 재로드

v2.0에서 `LeadAgent.session`이 앱 시작 시 1회만 로드되어, CLI로 `session update`해도 다음 세션에서 이전 맥락을 기억하지 못하는 문제가 있었습니다.

**해결:** `RunLeadSession` 시작 시 매번 디스크에서 `session.json` 재로드

### 5. InitProject 팀 복원

앱 재시작 후 `InitProject`로 프로젝트를 열면 기존 팀(`team.json`)을 무시하고 빈 상태로 시작하는 문제가 있었습니다.

**해결:** `InitProject`에서도 `LoadRolePlans()` → `buildTeamFromPlans()` 실행

### 6. 에이전트 상태 디스크 읽기

서브에이전트는 별도 CLI 프로세스로 실행되어 `.agent-status` 파일에 상태를 기록하지만, GUI의 `GetAgentStatuses()`는 메모리의 오래된 값을 반환하는 문제가 있었습니다.

**해결:** `GetAgentStatuses()` 호출 시 매번 디스크의 `.agent-status` 파일에서 최신 상태 읽기

### 7. 로그 색상 일관성

이모지 기반 색상 판별(`getLogColor`)이 불일치하여 같은 종류의 메시지가 다른 색상으로 표시되는 문제가 있었습니다.

**해결:** 오류(빨강), 경고(노랑), 상태 메시지(파랑)만 특별 색상, 나머지는 기본 텍스트색(`--text-primary`)으로 통일

---

## 코드 변경 범위

| 파일 | 변경 내용 |
|------|-----------|
| `gui/app.go` | `LogEvent`에 `Agent` 필드 추가, `log-append`에 agent 정보 전달, `GetAgentStatuses()` 디스크 읽기, `InitProject` 팀 복원 |
| `gui/cmd/claudestra/main.go` | `wait` 서브커맨드 추가 |
| `gui/internal/lead.go` | 세션 메모리 매번 재로드, 프롬프트에 `wait` 명령어 및 sleep 금지 규칙 추가 |
| `gui/frontend/src/App.tsx` | `LogEvent`에 `agent` 필드, `addLog`/`appendLog` agent 매칭, `handleInit`에 `refreshStatuses` 추가 |
| `gui/frontend/src/components/LogPanel.tsx` | `LogEntry`에 `agent` 필드, 서브에이전트 필터링, 색상 로직 단순화 |
| `gui/frontend/src/components/Sidebar.tsx` | RUNNING 에이전트 파란색 글로우 (box-shadow + border) |
| `gui/frontend/src/components/AgentDetailPanel.tsx` | 탭 순서 변경: 받은 지시 → 실행 로그 → 실행 결과 |

---

## CLI 명령어 (v2.1 업데이트)

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
claudestra wait <agent1> [agent2 ...]     # 비동기 작업 완료 대기 (v2.1 신규)
claudestra stop <agent|job-id>            # 실행 중인 작업 중지
claudestra lead-session '<request>'       # 단일 세션 모드 (팀장 자율 실행)
```

---

## 미결 사항 (v2.0에서 갱신)

- [x] CLI `wait` 명령 — sleep 폴링 제거 ✅ v2.1
- [x] 서브에이전트 로그 분리 — 중앙에서 제거, 상세 패널에서 확인 ✅ v2.1
- [x] 텍스트 꼬임 — agent별 append 매칭 ✅ v2.1
- [x] 세션 메모리 재로드 ✅ v2.1
- [x] InitProject 팀 복원 ✅ v2.1
- [x] 에이전트 상태 디스크 읽기 ✅ v2.1
- [ ] 경로 A 토큰 최적화 — GUI 승인 플로우의 Decompose+GenerateContract 통합
- [ ] Docker 이미지 내 Claude Code 인증 처리 방법
- [ ] 에이전트 비용 추적 (Claude Code 사용량 모니터링)
- [ ] 로그 가상화 — 장시간 세션에서 수천 개 로그 항목 렌더링 성능 (react-window 등)
