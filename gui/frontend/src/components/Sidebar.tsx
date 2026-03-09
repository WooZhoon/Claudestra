import { useState, useEffect } from 'react';

interface AgentStatus {
  id: string;
  role: string;
  status: string;
  isConsumer: boolean;
}

interface SidebarProps {
  agents: AgentStatus[];
  onSelectAgent: (id: string) => void;
  selectedAgent: string | null;
}

const statusIcon: Record<string, string> = {
  IDLE: '⚪',
  RUNNING: '🔵',
  DONE: '✅',
  ERROR: '❌',
};

export default function Sidebar({ agents, onSelectAgent, selectedAgent }: SidebarProps) {
  return (
    <div style={{
      width: 220,
      background: 'var(--bg-secondary)',
      borderRight: '1px solid var(--border)',
      display: 'flex',
      flexDirection: 'column',
      height: '100%',
    }}>
      <div style={{
        padding: '16px',
        borderBottom: '1px solid var(--border)',
        fontSize: 14,
        fontWeight: 600,
        color: 'var(--accent)',
      }}>
        🎼 Claudestra
      </div>

      <div style={{ padding: '8px', flex: 1, overflowY: 'auto' }}>
        <div style={{ fontSize: 11, color: 'var(--text-muted)', padding: '8px 8px 4px', textTransform: 'uppercase', letterSpacing: 1 }}>
          팀원
        </div>
        {agents.map(agent => (
          <div
            key={agent.id}
            onClick={() => onSelectAgent(agent.id)}
            style={{
              padding: '8px 12px',
              borderRadius: 6,
              cursor: 'pointer',
              marginBottom: 2,
              background: selectedAgent === agent.id ? 'var(--bg-tertiary)' : 'transparent',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              fontSize: 13,
            }}
          >
            <span>{statusIcon[agent.status] || '❓'}</span>
            <span style={{ flex: 1 }}>{agent.role}</span>
            <span style={{
              fontSize: 10,
              color: 'var(--text-muted)',
              background: 'var(--bg-primary)',
              padding: '2px 6px',
              borderRadius: 4,
            }}>
              {agent.isConsumer ? 'C' : 'P'}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
