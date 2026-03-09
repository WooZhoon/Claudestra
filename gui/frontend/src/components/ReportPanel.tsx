interface ReportPanelProps {
  report: string;
  visible: boolean;
  onClose: () => void;
}

export default function ReportPanel({ report, visible, onClose }: ReportPanelProps) {
  if (!visible || !report) return null;

  return (
    <div style={{
      position: 'absolute',
      top: 0,
      right: 0,
      width: '50%',
      height: '100%',
      background: 'var(--bg-secondary)',
      borderLeft: '1px solid var(--border)',
      display: 'flex',
      flexDirection: 'column',
      zIndex: 10,
    }}>
      <div style={{
        padding: '12px 16px',
        borderBottom: '1px solid var(--border)',
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
      }}>
        <span style={{ fontWeight: 600, color: 'var(--accent)' }}>📋 최종 보고서</span>
        <button
          onClick={onClose}
          style={{
            background: 'none',
            border: 'none',
            color: 'var(--text-muted)',
            cursor: 'pointer',
            fontSize: 18,
          }}
        >
          ✕
        </button>
      </div>
      <div style={{
        flex: 1,
        overflowY: 'auto',
        padding: '16px',
        fontSize: 13,
        lineHeight: 1.8,
        whiteSpace: 'pre-wrap',
        color: 'var(--text-secondary)',
      }}>
        {report}
      </div>
    </div>
  );
}
