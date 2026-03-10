"""Orchestra - Agent base classes.

Each agent runs Claude Code as an independent subprocess.
"""

import subprocess
import threading
import time
import json
from pathlib import Path
from dataclasses import dataclass, field
from enum import Enum


class AgentStatus(Enum):
    IDLE    = "idle"
    RUNNING = "running"
    DONE    = "done"
    ERROR   = "error"


@dataclass
class AgentConfig:
    agent_id:       str
    role:           str
    idea:           str          # system prompt (idea)
    work_dir:       Path
    read_refs:      list[Path] = field(default_factory=list)  # read-only reference paths
    contract:       str = ""     # interface contract
    allowed_tools:  list[str] = field(default_factory=list)   # allowed tools (empty = unrestricted)


class Agent:
    """Wraps a single Claude Code instance.

    Runs ``claude --print`` as a subprocess and captures the result.
    """

    def __init__(self, config: AgentConfig, lock_registry: "FileLockRegistry | None" = None):
        self.config  = config
        self.lock_registry = lock_registry
        self.status  = AgentStatus.IDLE
        self.output  = ""       # last execution result
        self._lock   = threading.Lock()

        # Create working directory
        self.config.work_dir.mkdir(parents=True, exist_ok=True)
        self._write_status("IDLE")

    # ── Public API ──────────────────────────────────────────────

    def run(self, instruction: str) -> str:
        """Execute an instruction and return the result (blocking)."""
        with self._lock:
            self.status = AgentStatus.RUNNING
            self._write_status("RUNNING")

        # Acquire work directory lock
        if self.lock_registry:
            from file_lock import LockConflictError
            try:
                self.lock_registry.acquire(
                    str(self.config.work_dir), self.config.agent_id
                )
                print(f"[{self.config.role}] 🔒 잠금 획득: {self.config.work_dir.name}/")
            except LockConflictError as e:
                self.status = AgentStatus.ERROR
                self._write_status("ERROR")
                self.output = f"LOCK CONFLICT: {e}"
                print(f"[{self.config.role}] 🔒 잠금 충돌: {e}")
                return self.output

        prompt = self._build_prompt(instruction)

        print(f"\n[{self.config.role}] 🚀 시작: {instruction[:60]}...")

        try:
            cmd = ["claude", "--print", "--dangerously-skip-permissions"]
            if self.config.allowed_tools:
                cmd += ["--allowedTools", ",".join(self.config.allowed_tools)]

            result = subprocess.run(
                cmd,
                input=prompt,
                cwd=str(self.config.work_dir),
                capture_output=True,
                text=True,
                timeout=300,  # 5분 타임아웃
            )

            if result.returncode == 0:
                self.output = result.stdout.strip()
                self.status = AgentStatus.DONE
                self._write_status("DONE")
                print(f"[{self.config.role}] ✅ 완료")
            else:
                self.output = result.stderr.strip()
                self.status = AgentStatus.ERROR
                self._write_status("ERROR")
                print(f"[{self.config.role}] ❌ 오류: {self.output[:100]}")

        except subprocess.TimeoutExpired:
            self.status = AgentStatus.ERROR
            self._write_status("ERROR")
            self.output = "TIMEOUT: 5분 초과"
            print(f"[{self.config.role}] ⏰ 타임아웃")

        except FileNotFoundError:
            self.status = AgentStatus.ERROR
            self._write_status("ERROR")
            self.output = "ERROR: 'claude' 명령어를 찾을 수 없습니다. Claude Code가 설치되어 있나요?"
            print(f"[{self.config.role}] ❌ {self.output}")

        finally:
            # Release lock after completion
            if self.lock_registry:
                self.lock_registry.release_all(self.config.agent_id)
                print(f"[{self.config.role}] 🔓 잠금 해제")

        return self.output

    def run_async(self, instruction: str) -> threading.Thread:
        """Run asynchronously and return the thread."""
        t = threading.Thread(target=self.run, args=(instruction,), daemon=True)
        t.start()
        return t

    def wait_until_done(self, timeout: float = 300) -> bool:
        """Poll until done. Returns True on success, False on timeout/error."""
        deadline = time.time() + timeout
        while time.time() < deadline:
            if self.status in (AgentStatus.DONE, AgentStatus.ERROR):
                return self.status == AgentStatus.DONE
            time.sleep(1)
        return False

    def reset(self) -> None:
        self.status = AgentStatus.IDLE
        self.output = ""
        self._write_status("IDLE")

    # ── Internal helpers ─────────────────────────────────────────────

    def _build_prompt(self, instruction: str) -> str:
        """Combine idea, cross-reference paths, and instruction into a prompt."""
        parts = [self.config.idea]

        parts.append("""[작업 원칙]
- 간결하게 핵심만 작성하세요. 전체 코드를 모두 작성하지 마세요.
- 핵심 구조, 주요 파일, 중요 로직만 구현하세요.
- 보일러플레이트나 반복적인 코드는 생략하고 주석으로 표시하세요.""")

        if self.config.contract:
            parts.append(f"[인터페이스 계약서 — 반드시 이 명세를 따르세요]\n{self.config.contract}")

        if self.config.read_refs:
            refs_text = "\n".join(f"  - {p}" for p in self.config.read_refs)
            parts.append(f"[읽기 전용 참조 디렉토리 — 수정 금지]\n{refs_text}")

        parts.append(f"[지시]\n{instruction}")
        return "\n\n".join(parts)

    def _write_status(self, status: str) -> None:
        status_file = self.config.work_dir / ".agent-status"
        status_file.write_text(status)
