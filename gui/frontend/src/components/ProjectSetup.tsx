import { useState } from 'react';

interface ProjectSetupProps {
  onInit: (dir: string, roles: string[]) => void;
  onOpen: (dir: string) => void;
  onSelectDir: () => Promise<string>;
}

const availableRoles = ['backend', 'frontend', 'db', 'reviewer', 'doc_writer'];

export default function ProjectSetup({ onInit, onOpen, onSelectDir }: ProjectSetupProps) {
  const [dir, setDir] = useState('');
  const [selectedRoles, setSelectedRoles] = useState<string[]>(['backend', 'frontend', 'db', 'reviewer']);

  const toggleRole = (role: string) => {
    setSelectedRoles(prev =>
      prev.includes(role) ? prev.filter(r => r !== role) : [...prev, role]
    );
  };

  const handleSelectDir = async () => {
    const selected = await onSelectDir();
    if (selected) setDir(selected);
  };

  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      height: '100%',
      background: 'var(--bg-primary)',
    }}>
      <div style={{
        background: 'var(--bg-secondary)',
        borderRadius: 12,
        padding: 32,
        width: 420,
        border: '1px solid var(--border)',
      }}>
        <h1 style={{ fontSize: 24, marginBottom: 4, color: 'var(--accent)' }}>🎼 Claudestra</h1>
        <p style={{ fontSize: 13, color: 'var(--text-muted)', marginBottom: 24 }}>
          Multi-Agent Orchestration System
        </p>

        <div style={{ marginBottom: 16 }}>
          <label style={{ fontSize: 12, color: 'var(--text-muted)', display: 'block', marginBottom: 6 }}>
            프로젝트 디렉토리
          </label>
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              value={dir}
              onChange={e => setDir(e.target.value)}
              placeholder="/workspace/myproject"
              style={{
                flex: 1,
                padding: '8px 12px',
                borderRadius: 6,
                border: '1px solid var(--border)',
                background: 'var(--bg-primary)',
                color: 'var(--text-primary)',
                fontSize: 13,
              }}
            />
            <button
              onClick={handleSelectDir}
              style={{
                padding: '8px 12px',
                borderRadius: 6,
                border: '1px solid var(--border)',
                background: 'var(--bg-tertiary)',
                color: 'var(--text-secondary)',
                cursor: 'pointer',
                fontSize: 13,
              }}
            >
              찾기
            </button>
          </div>
        </div>

        <div style={{ marginBottom: 20 }}>
          <label style={{ fontSize: 12, color: 'var(--text-muted)', display: 'block', marginBottom: 8 }}>
            팀원 구성
          </label>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            {availableRoles.map(role => (
              <button
                key={role}
                onClick={() => toggleRole(role)}
                style={{
                  padding: '6px 12px',
                  borderRadius: 6,
                  border: selectedRoles.includes(role) ? '1px solid var(--accent)' : '1px solid var(--border)',
                  background: selectedRoles.includes(role) ? 'var(--accent)' : 'var(--bg-tertiary)',
                  color: selectedRoles.includes(role) ? '#1a1b26' : 'var(--text-secondary)',
                  cursor: 'pointer',
                  fontSize: 12,
                  fontWeight: selectedRoles.includes(role) ? 600 : 400,
                }}
              >
                {role}
              </button>
            ))}
          </div>
        </div>

        <div style={{ display: 'flex', gap: 8 }}>
          <button
            onClick={() => dir && onInit(dir, selectedRoles)}
            disabled={!dir || selectedRoles.length === 0}
            style={{
              flex: 1,
              padding: '10px',
              borderRadius: 8,
              border: 'none',
              background: 'var(--accent)',
              color: '#1a1b26',
              fontSize: 14,
              fontWeight: 600,
              cursor: 'pointer',
              opacity: (!dir || selectedRoles.length === 0) ? 0.5 : 1,
            }}
          >
            새 프로젝트
          </button>
          <button
            onClick={() => dir && onOpen(dir)}
            disabled={!dir}
            style={{
              flex: 1,
              padding: '10px',
              borderRadius: 8,
              border: '1px solid var(--border)',
              background: 'var(--bg-tertiary)',
              color: 'var(--text-secondary)',
              fontSize: 14,
              cursor: 'pointer',
              opacity: !dir ? 0.5 : 1,
            }}
          >
            기존 프로젝트 열기
          </button>
        </div>
      </div>
    </div>
  );
}
