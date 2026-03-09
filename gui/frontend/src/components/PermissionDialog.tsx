import { useState, useEffect, useCallback } from 'react';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import * as WailsApp from '../../wailsjs/go/main/App';

interface PermissionRequest {
  id: string;
  tool: string;
  command: string;
  agent: string;
  timestamp: string;
}

export default function PermissionDialog() {
  const [requests, setRequests] = useState<PermissionRequest[]>([]);

  useEffect(() => {
    const cancel = EventsOn('permission-request', (req: PermissionRequest) => {
      setRequests(prev => [...prev, req]);
    });
    return () => cancel();
  }, []);

  const handleRespond = useCallback(async (id: string, allowed: boolean) => {
    try {
      await WailsApp.RespondPermission(id, allowed);
    } catch {}
    setRequests(prev => prev.filter(r => r.id !== id));
  }, []);

  if (requests.length === 0) return null;

  const current = requests[0];

  return (
    <div style={{
      position: 'fixed',
      top: 0,
      left: 0,
      right: 0,
      bottom: 0,
      background: 'rgba(0,0,0,0.6)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      zIndex: 9999,
    }}>
      <div style={{
        background: 'var(--bg-secondary, #1e1f2e)',
        border: '1px solid var(--border, #333)',
        borderRadius: 8,
        padding: '20px 24px',
        maxWidth: 520,
        width: '90%',
        boxShadow: '0 8px 32px rgba(0,0,0,0.4)',
      }}>
        <div style={{
          fontSize: 15,
          fontWeight: 600,
          marginBottom: 12,
          color: 'var(--text-primary, #e0e0e0)',
        }}>
          Permission Request
          {requests.length > 1 && (
            <span style={{ fontWeight: 400, fontSize: 12, marginLeft: 8, color: 'var(--text-muted, #888)' }}>
              (+{requests.length - 1} more)
            </span>
          )}
        </div>

        <div style={{
          fontSize: 12,
          color: 'var(--text-muted, #888)',
          marginBottom: 8,
        }}>
          {current.tool}{current.agent && ` (${current.agent})`}
        </div>

        <div style={{
          background: 'var(--bg-tertiary, #16171f)',
          border: '1px solid var(--border, #333)',
          borderRadius: 4,
          padding: '10px 12px',
          fontSize: 13,
          fontFamily: 'monospace',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
          maxHeight: 200,
          overflow: 'auto',
          marginBottom: 16,
          color: 'var(--text-primary, #e0e0e0)',
        }}>
          {current.command}
        </div>

        <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
          <button
            onClick={() => handleRespond(current.id, false)}
            style={{
              padding: '8px 20px',
              borderRadius: 4,
              border: '1px solid #e53e3e',
              background: 'transparent',
              color: '#e53e3e',
              cursor: 'pointer',
              fontSize: 13,
              fontWeight: 500,
            }}
          >
            Disallow
          </button>
          <button
            onClick={() => handleRespond(current.id, true)}
            style={{
              padding: '8px 20px',
              borderRadius: 4,
              border: 'none',
              background: '#38a169',
              color: '#fff',
              cursor: 'pointer',
              fontSize: 13,
              fontWeight: 500,
            }}
          >
            Allow
          </button>
        </div>
      </div>
    </div>
  );
}
