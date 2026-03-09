import { useState, KeyboardEvent } from 'react';

interface InputBarProps {
  onSubmit: (input: string) => void;
  disabled: boolean;
}

export default function InputBar({ onSubmit, disabled }: InputBarProps) {
  const [input, setInput] = useState('');

  const handleSubmit = () => {
    const trimmed = input.trim();
    if (!trimmed || disabled) return;
    onSubmit(trimmed);
    setInput('');
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <div style={{
      padding: '12px 16px',
      borderTop: '1px solid var(--border)',
      background: 'var(--bg-secondary)',
      display: 'flex',
      gap: 8,
    }}>
      <input
        value={input}
        onChange={e => setInput(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={disabled ? '실행 중...' : '요청을 입력하세요...'}
        disabled={disabled}
        style={{
          flex: 1,
          padding: '10px 14px',
          borderRadius: 8,
          border: '1px solid var(--border)',
          background: 'var(--bg-primary)',
          color: 'var(--text-primary)',
          fontSize: 14,
          outline: 'none',
        }}
      />
      <button
        onClick={handleSubmit}
        disabled={disabled || !input.trim()}
        style={{
          padding: '10px 20px',
          borderRadius: 8,
          border: 'none',
          background: disabled ? 'var(--bg-tertiary)' : 'var(--accent)',
          color: disabled ? 'var(--text-muted)' : '#1a1b26',
          fontSize: 14,
          fontWeight: 600,
          cursor: disabled ? 'not-allowed' : 'pointer',
        }}
      >
        전송
      </button>
    </div>
  );
}
