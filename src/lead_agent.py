"""
Orchestra - Lead Agent (팀장)
사용자 입력을 분석해서 각 팀원에게 태스크를 배분합니다.
"""

import subprocess
import json
import re
from collections import deque
from pathlib import Path
from typing import Optional

from agent import Agent, AgentConfig, AgentStatus


class LeadAgent:
    """
    팀장 AI.
    1. 사용자 입력을 받아 태스크를 분해합니다.
    2. 각 팀원에게 적절한 지시를 배분합니다.
    3. 팀원 완료 후 결과를 수집하고 보고합니다.
    """

    IDEA = """당신은 소프트웨어 개발 팀의 팀장 AI입니다.
당신의 역할은:
1. 사용자의 요구사항을 분석합니다.
2. 각 팀원의 역할에 맞게 태스크를 분해합니다.
3. 각 태스크 간의 의존관계(depends_on)를 명시합니다.

중요: 개발 태스크가 아닌 경우(인사, 잡담, 질문 등)에는 빈 배열 []을 반환하세요.
반드시 JSON 형식으로만 응답하세요. 다른 텍스트는 포함하지 마세요."""

    def __init__(self, work_dir: Path):
        self.work_dir  = work_dir
        self.agents: dict[str, Agent] = {}
        self.work_dir.mkdir(parents=True, exist_ok=True)

        # 대화 메모리: 마지막 실행 컨텍스트 (토큰 절약을 위해 요약만 저장)
        self._last_context: str | None = None   # 이전 보고서 핵심 요약

    # ── 팀원 관리 ──────────────────────────────────────────────

    def add_agent(self, agent: Agent):
        self.agents[agent.config.agent_id] = agent
        print(f"[팀장] 팀원 추가: {agent.config.role} (id={agent.config.agent_id})")

    def list_agents(self) -> list[dict]:
        return [
            {
                "id":     a.config.agent_id,
                "role":   a.config.role,
                "status": a.status.value,
            }
            for a in self.agents.values()
        ]

    # ── 핵심: 사용자 입력 처리 ─────────────────────────────────

    def process(self, user_input: str) -> str:
        """
        사용자 입력 → 계획 수립 → 사용자 승인 → 단계별 실행 → 최종 보고
        """
        print(f"\n{'='*60}")
        print(f"[팀장] 사용자 입력 수신: {user_input}")
        print(f"{'='*60}")

        # 1단계: 계획 수립
        print("\n[팀장] 📋 실행 계획 수립 중...")
        plan = self._decompose(user_input)
        if plan is None:
            return "계획 수립에 실패했습니다."
        if len(plan) == 0:
            print("[팀장] 💬 개발 태스크가 아닙니다. 팀장이 직접 응답합니다.")
            return self._direct_reply(user_input)

        # 2단계: 인터페이스 계약서 생성
        print("\n[팀장] 📜 인터페이스 계약서 작성 중...")
        contract = self._generate_contract(user_input, plan)

        # 3단계: 계획 + 계약서 표시 & 사용자 승인
        self._print_plan(plan)
        if contract:
            self._print_contract(contract)
        approval = input("\n진행할까요? (y: 전체 실행 / s: 단계별 확인 / n: 취소) > ").strip().lower()

        if approval == "n":
            return "작업이 취소되었습니다."

        step_by_step = (approval == "s")

        # 계약서 저장 & 에이전트에 주입
        if contract:
            self._save_contract(contract)
            for agent in self.agents.values():
                agent.config.contract = contract

        # 4단계: 단계별 실행
        all_results: dict[str, str] = {}
        for step in plan:
            step_num   = step.get("step", "?")
            step_title = step.get("title", "")
            tasks      = step.get("tasks", [])

            print(f"\n{'─'*60}")
            print(f"📌 {step_num}단계: {step_title}")
            print(f"{'─'*60}")

            if step_by_step:
                confirm = input(f"  이 단계를 실행할까요? (y/n/skip) > ").strip().lower()
                if confirm == "n":
                    print("  작업이 취소되었습니다.")
                    break
                if confirm == "skip":
                    print("  이 단계를 건너뜁니다.")
                    continue

            results = self._execute_step(tasks)
            all_results.update(results)

            # 단계 완료 요약
            done  = sum(1 for aid in results if self.agents.get(aid) and self.agents[aid].status == AgentStatus.DONE)
            total = len(tasks)
            print(f"\n  ✅ {step_num}단계 완료 ({done}/{total} 성공)")

        # 4단계: 최종 보고
        if all_results:
            print(f"\n[팀장] 📝 최종 보고 작성 중...")
            return self._summarize(user_input, all_results)
        else:
            return "실행된 태스크가 없습니다."

    def _print_plan(self, plan: list[dict]):
        """실행 계획을 사용자에게 보여줍니다."""
        print(f"\n{'='*60}")
        print("📋 실행 계획")
        print(f"{'='*60}")
        for step in plan:
            step_num   = step.get("step", "?")
            step_title = step.get("title", "")
            tasks      = step.get("tasks", [])
            print(f"\n  📌 {step_num}단계: {step_title}")
            for t in tasks:
                agent_id = t.get("agent_id", "?")
                desc     = t.get("instruction", "")[:70]
                print(f"     [{agent_id}] {desc}...")
        print(f"\n{'='*60}")

    # ── 내부: 대화 메모리 ──────────────────────────────────────

    def _save_context(self, user_input: str, report: str):
        """보고서에서 핵심 요약만 추출하여 컨텍스트로 저장합니다."""
        # 보고서를 간결하게 요약 (토큰 절약)
        prompt = f"""아래 보고서에서 핵심 정보만 추출하여 5줄 이내로 요약하세요.
반드시 포함: 1) 무엇을 만들었는지 2) 해결이 필요한 문제 목록 3) 다음에 해야 할 일
다른 텍스트 없이 요약만 출력하세요.

[사용자 요청]
{user_input}

[보고서]
{report[:2000]}"""

        try:
            result = subprocess.run(
                ["claude", "--print", "--dangerously-skip-permissions", prompt],
                cwd=str(self.work_dir),
                capture_output=True,
                text=True,
                timeout=60,
            )
            if result.returncode == 0:
                self._last_context = result.stdout.strip()
                return
        except (FileNotFoundError, subprocess.TimeoutExpired):
            pass

        # 폴백: 보고서 앞부분만 저장
        self._last_context = f"[이전 요청: {user_input}]\n{report[:500]}"

    def _build_context_block(self) -> str:
        """이전 대화 컨텍스트를 프롬프트 블록으로 반환합니다."""
        if not self._last_context:
            return ""
        return f"\n[이전 작업 컨텍스트 — 사용자가 이전 작업을 참조할 수 있습니다]\n{self._last_context}\n"

    # ── 내부: 태스크 분해 ──────────────────────────────────────

    def _decompose(self, user_input: str) -> list[dict]:
        """
        Claude Code를 사용해 user_input을 의존성 기반 태스크 그래프로 분해합니다.
        토폴로지 정렬로 실행 웨이브(step)를 자동 생성합니다.
        """
        agent_list = json.dumps(self.list_agents(), ensure_ascii=False, indent=2)

        context_block = self._build_context_block()

        prompt = f"""{self.IDEA}

[현재 팀원 목록]
{agent_list}
{context_block}
[사용자 요구사항]
{user_input}

중요 규칙:
- 개발/구현/설계 등 실제 작업이 필요한 요청만 태스크로 분해하세요.
- 인사, 잡담, 단순 질문 등 개발 태스크가 아닌 경우 반드시 빈 배열 []만 반환하세요.
- "안녕", "뭐해?", "고마워" 같은 입력에는 절대 태스크를 생성하지 마세요. []를 반환하세요.
- 단, "이전 작업 컨텍스트"가 있고, 사용자가 "고쳐줘", "수정해", "해결해" 등 이전 작업을 참조하는 경우에는 개발 태스크입니다. 컨텍스트의 문제점을 기반으로 태스크를 분해하세요.

작업 계획 규칙:
- 각 태스크에 고유 id를 부여하세요 (예: "t1", "t2", ...).
- depends_on에 선행 태스크 id를 명시하세요. 의존성이 없으면 빈 배열 [].
- 시스템이 의존성을 분석하여 자동으로 병렬 실행합니다. 단계 번호는 불필요합니다.
- 각 instruction은 간결하게 핵심만. 전체 코드가 아닌 핵심 구조만 지시하세요.

개발 태스크인 경우에만 아래 형식으로 응답하세요:
[
  {{"id": "t1", "agent_id": "팀원ID", "instruction": "구체적인 지시", "depends_on": []}},
  {{"id": "t2", "agent_id": "팀원ID", "instruction": "구체적인 지시", "depends_on": ["t1"]}},
  {{"id": "t3", "agent_id": "팀원ID", "instruction": "구체적인 지시", "depends_on": ["t1"]}},
  {{"id": "t4", "agent_id": "팀원ID", "instruction": "구체적인 지시", "depends_on": ["t2", "t3"]}}
]"""

        try:
            result = subprocess.run(
                ["claude", "--print", "--dangerously-skip-permissions", prompt],
                cwd=str(self.work_dir),
                capture_output=True,
                text=True,
                timeout=120,
            )

            if result.returncode != 0:
                print(f"[팀장] ❌ 태스크 분해 오류: {result.stderr[:200]}")
                return self._fallback_decompose(user_input)

            raw = result.stdout.strip()
            raw = re.sub(r"```json\s*|\s*```", "", raw).strip()
            tasks = json.loads(raw)

            # 빈 배열이면 그대로 반환 (개발 태스크 아님)
            if isinstance(tasks, list) and len(tasks) == 0:
                return []

            # 유효한 에이전트만 남기기
            valid_ids = set(self.agents.keys())
            tasks = [t for t in tasks if t.get("agent_id") in valid_ids]

            if not tasks:
                return []

            # 토폴로지 정렬 → step 기반 포맷으로 변환
            return self._toposort_to_steps(tasks)

        except (json.JSONDecodeError, FileNotFoundError) as e:
            print(f"[팀장] ⚠️  파싱 실패 ({e}), 폴백 모드 사용")
            return self._fallback_decompose(user_input)

    def _toposort_to_steps(self, tasks: list[dict]) -> list[dict]:
        """
        의존성 기반 태스크 리스트를 토폴로지 정렬하여
        실행 웨이브(step) 포맷으로 변환합니다 (Kahn's algorithm).
        """
        task_map = {t["id"]: t for t in tasks}
        valid_ids = set(task_map.keys())

        # 진입 차수 계산
        in_degree: dict[str, int] = {tid: 0 for tid in valid_ids}
        dependents: dict[str, list[str]] = {tid: [] for tid in valid_ids}

        for t in tasks:
            for dep in t.get("depends_on", []):
                if dep in valid_ids:
                    in_degree[t["id"]] += 1
                    dependents[dep].append(t["id"])

        # BFS 웨이브별 수집
        queue = deque([tid for tid, deg in in_degree.items() if deg == 0])
        waves: list[list[dict]] = []
        visited = set()

        while queue:
            wave_size = len(queue)
            wave_tasks = []
            for _ in range(wave_size):
                tid = queue.popleft()
                visited.add(tid)
                wave_tasks.append(task_map[tid])
                for dep_tid in dependents[tid]:
                    in_degree[dep_tid] -= 1
                    if in_degree[dep_tid] == 0:
                        queue.append(dep_tid)
            waves.append(wave_tasks)

        # 순환 의존성 검출 — 방문하지 못한 노드가 있으면 순환
        if len(visited) < len(valid_ids):
            orphans = valid_ids - visited
            print(f"[팀장] ⚠️  순환 의존성 감지: {orphans}, 남은 태스크를 마지막 웨이브에 추가")
            waves.append([task_map[tid] for tid in orphans])

        # step 포맷으로 변환
        steps = []
        for i, wave in enumerate(waves, 1):
            agent_names = ", ".join(t["agent_id"] for t in wave)
            steps.append({
                "step": i,
                "title": f"Wave {i} ({agent_names})",
                "tasks": [
                    {"agent_id": t["agent_id"], "instruction": t["instruction"]}
                    for t in wave
                ],
            })
        return steps

    def _fallback_decompose(self, user_input: str) -> list[dict]:
        """Claude Code 없이 동작하는 폴백: 모든 팀원에게 동일 지시."""
        print("[팀장] ⚠️  폴백: 모든 팀원에게 동일 지시 전달")
        return [
            {
                "step": 1,
                "title": "전체 작업",
                "tasks": [
                    {
                        "agent_id":    agent_id,
                        "instruction": f"다음 요구사항에서 당신의 역할({agent.config.role})에 해당하는 부분을 수행하세요:\n\n{user_input}",
                    }
                    for agent_id, agent in self.agents.items()
                ],
            }
        ]

    # ── 내부: 인터페이스 계약서 ──────────────────────────────────

    def _generate_contract(self, user_input: str, plan: list[dict]) -> str | None:
        """팀원들이 공유할 인터페이스 계약서를 생성합니다."""
        agent_list = json.dumps(self.list_agents(), ensure_ascii=False, indent=2)
        plan_text = json.dumps(plan, ensure_ascii=False, indent=2)

        prompt = f"""당신은 소프트웨어 아키텍트입니다.
아래 프로젝트 계획을 보고, 모든 팀원이 따라야 할 인터페이스 계약서를 YAML 형식으로 작성하세요.

[사용자 요구사항]
{user_input}

[실행 계획]
{plan_text}

[팀원 목록]
{agent_list}

계약서에 반드시 포함할 내용:
1. tech_stack: 사용할 언어, 프레임워크, DB 엔진
2. naming_conventions: 필드명 규칙 (camelCase/snake_case), 언어 (한글/영문)
3. api_endpoints: 주요 엔드포인트 목록 (method, path, request/response 필드명)
4. db_schema: 주요 테이블/컬렉션과 필드명
5. shared_types: 공유 타입 정의 (예: User, Course 등)

규칙:
- 간결하게 핵심만 작성하세요. 50줄 이내로.
- YAML만 출력하세요. 마크다운 펜스(```)나 설명 텍스트는 포함하지 마세요."""

        try:
            result = subprocess.run(
                ["claude", "--print", "--dangerously-skip-permissions", prompt],
                cwd=str(self.work_dir),
                capture_output=True,
                text=True,
                timeout=120,
            )
            if result.returncode == 0:
                raw = result.stdout.strip()
                raw = re.sub(r"```ya?ml\s*|\s*```", "", raw).strip()
                return raw
        except (FileNotFoundError, subprocess.TimeoutExpired):
            pass

        print("[팀장] ⚠️  계약서 생성 실패, 계약서 없이 진행합니다.")
        return None

    def _print_contract(self, contract: str):
        """인터페이스 계약서를 표시합니다."""
        print(f"\n{'='*60}")
        print("📜 인터페이스 계약서")
        print(f"{'='*60}")
        print(contract)
        print(f"{'='*60}")

    def _save_contract(self, contract: str):
        """계약서를 파일로 저장합니다."""
        contracts_dir = self.work_dir / ".orchestra" / "contracts"
        contracts_dir.mkdir(parents=True, exist_ok=True)
        contract_file = contracts_dir / "contract.yaml"
        contract_file.write_text(contract)
        print(f"[팀장] 📜 계약서 저장: {contract_file}")

    # ── 내부: 팀장 직접 응답 ────────────────────────────────────

    def _direct_reply(self, user_input: str) -> str:
        """개발 태스크가 아닌 경우 팀장이 직접 응답합니다."""
        context_block = self._build_context_block()

        prompt = f"""당신은 소프트웨어 개발 팀의 팀장입니다.
사용자의 메시지에 친절하게 한국어로 응답하세요.
개발 관련 요청이 필요하면 어떤 것을 도와줄 수 있는지 안내해주세요.
{context_block}
[사용자 메시지]
{user_input}"""

        try:
            result = subprocess.run(
                ["claude", "--print", "--dangerously-skip-permissions", prompt],
                cwd=str(self.work_dir),
                capture_output=True,
                text=True,
                timeout=60,
            )
            if result.returncode == 0:
                return result.stdout.strip()
        except FileNotFoundError:
            pass

        return "안녕하세요! 개발 관련 요청을 입력해주시면 팀원들에게 배분하여 처리하겠습니다."

    # ── 내부: 단계 실행 ──────────────────────────────────────

    def _execute_step(self, tasks: list[dict]) -> dict[str, str]:
        """한 단계의 태스크들을 병렬 실행하고 결과를 반환합니다."""
        results: dict[str, str] = {}
        threads: list[tuple[str, object]] = []

        for task in tasks:
            agent_id    = task["agent_id"]
            instruction = task["instruction"]
            agent       = self.agents.get(agent_id)
            if not agent:
                print(f"  ⚠️  알 수 없는 에이전트: {agent_id}")
                continue

            agent.reset()
            t = agent.run_async(instruction)
            threads.append((agent_id, t))

        for aid, t in threads:
            t.join(timeout=300)
            results[aid] = self.agents[aid].output

        return results

    # ── 내부: 결과 요약 ────────────────────────────────────────

    def _summarize(self, user_input: str, results: dict[str, str]) -> str:
        """팀원 결과물을 취합해 최종 보고서를 작성합니다."""
        results_text = "\n\n".join(
            f"[{self.agents[aid].config.role} 결과]\n{output}"
            for aid, output in results.items()
            if aid in self.agents
        )

        prompt = f"""당신은 소프트웨어 개발 팀의 팀장입니다.
팀원들의 작업 결과를 취합하여 사용자에게 전달할 최종 보고서를 작성하세요.

[원래 사용자 요청]
{user_input}

[팀원별 작업 결과]
{results_text}

위 내용을 바탕으로:
1. 완료된 작업 요약
2. 각 팀원이 수행한 내용
3. 전체 결과물에 대한 평가
4. 다음 단계 제안

형식으로 한국어 보고서를 작성하세요."""

        try:
            result = subprocess.run(
                ["claude", "--print", "--dangerously-skip-permissions", prompt],
                cwd=str(self.work_dir),
                capture_output=True,
                text=True,
                timeout=120,
            )
            if result.returncode == 0:
                report = result.stdout.strip()
                self._save_context(user_input, report)
                return report
        except FileNotFoundError:
            pass

        # 폴백: 단순 결합
        fallback = f"[팀원 결과 요약]\n\n{results_text}"
        self._save_context(user_input, fallback)
        return fallback
