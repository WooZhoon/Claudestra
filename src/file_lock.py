"""
Claudestra - File Lock Registry
에이전트 간 파일/디렉토리 충돌을 방지하는 잠금 레지스트리
"""

import threading
import json
from pathlib import Path


class LockConflictError(Exception):
    def __init__(self, path: str, holder: str, requester: str):
        self.path = path
        self.holder = holder
        self.requester = requester
        super().__init__(
            f"'{path}' 는 '{holder}' 에이전트가 사용 중 (요청: '{requester}')"
        )


class FileLockRegistry:
    """
    Thread-safe 파일 잠금 레지스트리.
    여러 에이전트가 동일 파일/디렉토리에 동시 쓰기하는 것을 방지합니다.
    잠금 상태는 .orchestra/locks/registry.json 에 저장됩니다.
    """

    def __init__(self, locks_dir: Path):
        self._locks_dir = locks_dir
        self._locks: dict[str, str] = {}  # normalized_path → agent_id
        self._mu = threading.Lock()
        self._locks_dir.mkdir(parents=True, exist_ok=True)
        self._load()

    def acquire(self, file_path: str, agent_id: str) -> None:
        """잠금을 획득합니다. 다른 에이전트가 보유 중이면 LockConflictError."""
        with self._mu:
            normalized = str(Path(file_path).resolve())
            holder = self._locks.get(normalized)
            if holder is not None and holder != agent_id:
                raise LockConflictError(normalized, holder, agent_id)
            self._locks[normalized] = agent_id
            self._save()

    def release(self, file_path: str, agent_id: str) -> None:
        """잠금을 해제합니다. 보유자만 해제 가능."""
        with self._mu:
            normalized = str(Path(file_path).resolve())
            if self._locks.get(normalized) == agent_id:
                del self._locks[normalized]
                self._save()

    def release_all(self, agent_id: str) -> None:
        """특정 에이전트의 모든 잠금을 해제합니다 (작업 완료 후 정리용)."""
        with self._mu:
            self._locks = {
                path: holder
                for path, holder in self._locks.items()
                if holder != agent_id
            }
            self._save()

    def held_by(self, file_path: str) -> str | None:
        """잠금 보유자를 반환합니다. 없으면 None."""
        with self._mu:
            normalized = str(Path(file_path).resolve())
            return self._locks.get(normalized)

    def list_locks(self) -> dict[str, str]:
        """현재 모든 잠금의 스냅샷을 반환합니다."""
        with self._mu:
            return dict(self._locks)

    def _save(self):
        registry_file = self._locks_dir / "registry.json"
        registry_file.write_text(json.dumps(self._locks, indent=2, ensure_ascii=False))

    def _load(self):
        registry_file = self._locks_dir / "registry.json"
        if registry_file.exists():
            try:
                self._locks = json.loads(registry_file.read_text())
            except json.JSONDecodeError:
                self._locks = {}
