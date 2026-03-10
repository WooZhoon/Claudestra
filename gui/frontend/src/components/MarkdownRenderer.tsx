import { memo } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { Components } from 'react-markdown';

interface MarkdownRendererProps {
  content: string;
  style?: React.CSSProperties;
}

const components: Components = {
  h1: ({ children }) => (
    <h1 style={{ fontSize: 22, fontWeight: 700, color: 'var(--text-primary)', borderBottom: '1px solid var(--border)', paddingBottom: 8, margin: '16px 0 12px' }}>
      {children}
    </h1>
  ),
  h2: ({ children }) => (
    <h2 style={{ fontSize: 18, fontWeight: 600, color: 'var(--text-primary)', borderBottom: '1px solid var(--border)', paddingBottom: 6, margin: '14px 0 10px' }}>
      {children}
    </h2>
  ),
  h3: ({ children }) => (
    <h3 style={{ fontSize: 15, fontWeight: 600, color: 'var(--text-primary)', margin: '12px 0 8px' }}>
      {children}
    </h3>
  ),
  h4: ({ children }) => (
    <h4 style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)', margin: '10px 0 6px' }}>
      {children}
    </h4>
  ),
  p: ({ children }) => (
    <p style={{ margin: '8px 0', lineHeight: 1.7 }}>{children}</p>
  ),
  a: ({ href, children }) => (
    <a href={href} style={{ color: 'var(--accent)' }} target="_blank" rel="noopener noreferrer">
      {children}
    </a>
  ),
  strong: ({ children }) => (
    <strong style={{ color: 'var(--text-primary)' }}>{children}</strong>
  ),
  blockquote: ({ children }) => (
    <blockquote style={{ borderLeft: '3px solid var(--accent)', paddingLeft: 12, margin: '8px 0', color: 'var(--text-muted)' }}>
      {children}
    </blockquote>
  ),
  ul: ({ children }) => (
    <ul style={{ paddingLeft: 20, margin: '6px 0' }}>{children}</ul>
  ),
  ol: ({ children }) => (
    <ol style={{ paddingLeft: 20, margin: '6px 0' }}>{children}</ol>
  ),
  li: ({ children }) => (
    <li style={{ margin: '3px 0', lineHeight: 1.6 }}>{children}</li>
  ),
  code: ({ className, children }) => {
    const isBlock = className?.startsWith('language-');
    if (isBlock) {
      return (
        <code style={{
          display: 'block',
          background: 'var(--bg-tertiary)',
          border: '1px solid var(--border)',
          borderRadius: 6,
          padding: 12,
          fontSize: 12,
          overflowX: 'auto',
          fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
        }}>
          {children}
        </code>
      );
    }
    return (
      <code style={{
        background: 'var(--bg-tertiary)',
        padding: '2px 6px',
        borderRadius: 3,
        fontSize: 12,
        fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
      }}>
        {children}
      </code>
    );
  },
  pre: ({ children }) => (
    <pre style={{ margin: '8px 0', overflowX: 'auto' }}>{children}</pre>
  ),
  table: ({ children }) => (
    <table style={{ borderCollapse: 'collapse', width: '100%', margin: '8px 0' }}>
      {children}
    </table>
  ),
  th: ({ children }) => (
    <th style={{ border: '1px solid var(--border)', padding: '6px 10px', textAlign: 'left', fontWeight: 600, color: 'var(--text-primary)' }}>
      {children}
    </th>
  ),
  td: ({ children }) => (
    <td style={{ border: '1px solid var(--border)', padding: '6px 10px' }}>
      {children}
    </td>
  ),
};

export default memo(function MarkdownRenderer({ content, style }: MarkdownRendererProps) {
  return (
    <div style={style}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {content}
      </ReactMarkdown>
    </div>
  );
});
