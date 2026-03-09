import { useRef, useEffect } from 'react';

interface LogPanelProps {
  logs: string[];
}

export default function LogPanel({ logs }: LogPanelProps) {
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  return (
    <div style={{
      flex: 1,
      background: 'var(--bg-primary)',
      overflowY: 'auto',
      padding: '12px 16px',
      fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
      fontSize: 13,
      lineHeight: 1.6,
    }}>
      {logs.map((log, i) => (
        <div key={i} style={{
          color: log.includes('❌') || log.includes('오류')
            ? 'var(--error)'
            : log.includes('✅') || log.includes('완료')
            ? 'var(--success)'
            : log.includes('📌') || log.includes('📋')
            ? 'var(--accent)'
            : log.includes('⚠️')
            ? 'var(--warning)'
            : 'var(--text-secondary)',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
        }}>
          {log}
        </div>
      ))}
      <div ref={bottomRef} />
    </div>
  );
}
