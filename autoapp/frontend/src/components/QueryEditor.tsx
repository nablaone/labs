import React, { useCallback, useEffect, useRef, useState } from 'react'
import { useQueryStore } from '../store/queryStore'

const SUGGESTIONS = [
  'SELECT * FROM orders',
  'SELECT * FROM customers',
  'SELECT o.*, c.name FROM orders o JOIN customers c ON c.id = o.customer_id',
  'SELECT status, COUNT(*), SUM(amount) FROM orders GROUP BY status',
  'SELECT DATE_TRUNC(\'day\', created_at) AS day, COUNT(*) FROM orders GROUP BY day ORDER BY day',
]

export default function QueryEditor() {
  const { sql, setSql, loading, result, history, pushHistory, setResult, setLoading } = useQueryStore()
  const [showHistory, setShowHistory] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const runQuery = useCallback(async (querySql = sql) => {
    if (!querySql.trim()) return
    setLoading(true)
    setResult(null)
    try {
      const res = await fetch('/query', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sql: querySql }),
      })
      const data = await res.json()
      setResult(data)
      if (!data.error) pushHistory(querySql)
    } catch (e) {
      setResult({ columns: [], rows: [], error: String(e) })
    } finally {
      setLoading(false)
    }
  }, [sql, setLoading, setResult, pushHistory])

  const onKeyDown = (e: React.KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault()
      runQuery()
    }
  }

  const pickHistory = (h: string) => {
    setSql(h)
    setShowHistory(false)
    textareaRef.current?.focus()
  }

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setShowHistory(false)
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [])

  return (
    <div style={styles.wrap}>
      <div style={styles.editorRow}>
        <textarea
          ref={textareaRef}
          style={styles.textarea}
          value={sql}
          onChange={(e) => setSql(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder="Enter SQL… (Ctrl+Enter to run)"
          spellCheck={false}
          rows={4}
        />
        <div style={styles.buttonCol}>
          <button style={{ ...styles.btn, ...styles.btnPrimary }} onClick={() => runQuery()} disabled={loading}>
            {loading ? '…' : '▶ Run'}
          </button>
          <button
            style={{ ...styles.btn, ...styles.btnSecondary }}
            onClick={() => setShowHistory((v) => !v)}
          >
            History
          </button>
        </div>
      </div>

      {showHistory && (
        <div style={styles.historyPanel}>
          <div style={styles.historyHeader}>
            <span>Recent queries</span>
            <button style={styles.closeBtn} onClick={() => setShowHistory(false)}>✕</button>
          </div>
          {SUGGESTIONS.filter((s) => !history.includes(s)).map((s) => (
            <div key={s} style={styles.historyItem} onClick={() => pickHistory(s)}>
              <span style={styles.badge}>example</span> {s}
            </div>
          ))}
          {history.map((h) => (
            <div key={h} style={styles.historyItem} onClick={() => pickHistory(h)}>{h}</div>
          ))}
          {history.length === 0 && SUGGESTIONS.length === 0 && (
            <div style={{ padding: '8px 12px', color: '#64748b' }}>No history yet</div>
          )}
        </div>
      )}

      {result?.error && (
        <div style={styles.errorBanner}>{result.error}</div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  wrap: { display: 'flex', flexDirection: 'column', gap: 0, position: 'relative' },
  editorRow: { display: 'flex', gap: 8, alignItems: 'flex-start' },
  textarea: {
    flex: 1,
    background: '#1e2433',
    color: '#e2e8f0',
    border: '1px solid #2d3748',
    borderRadius: 6,
    padding: '10px 12px',
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
    fontSize: 13,
    resize: 'vertical',
    outline: 'none',
    lineHeight: 1.5,
  },
  buttonCol: { display: 'flex', flexDirection: 'column', gap: 6, minWidth: 80 },
  btn: {
    padding: '8px 16px',
    borderRadius: 6,
    border: 'none',
    cursor: 'pointer',
    fontSize: 13,
    fontWeight: 600,
    transition: 'background 0.15s',
  },
  btnPrimary: { background: '#6366f1', color: '#fff' },
  btnSecondary: { background: '#2d3748', color: '#94a3b8' },
  historyPanel: {
    position: 'absolute',
    top: '100%',
    left: 0,
    right: 80 + 8,
    background: '#1a2030',
    border: '1px solid #2d3748',
    borderRadius: 6,
    zIndex: 100,
    maxHeight: 320,
    overflowY: 'auto',
    marginTop: 4,
    boxShadow: '0 8px 24px rgba(0,0,0,0.4)',
  },
  historyHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: '8px 12px',
    borderBottom: '1px solid #2d3748',
    fontSize: 11,
    color: '#64748b',
    textTransform: 'uppercase',
    letterSpacing: 1,
  },
  closeBtn: {
    background: 'none',
    border: 'none',
    color: '#64748b',
    cursor: 'pointer',
    fontSize: 14,
  },
  historyItem: {
    padding: '8px 12px',
    cursor: 'pointer',
    fontSize: 12,
    fontFamily: 'monospace',
    borderBottom: '1px solid #1e2433',
    color: '#94a3b8',
    whiteSpace: 'pre',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  badge: {
    background: '#1e3a5f',
    color: '#60a5fa',
    padding: '1px 5px',
    borderRadius: 3,
    fontSize: 10,
    marginRight: 6,
  },
  errorBanner: {
    background: '#3b1f1f',
    border: '1px solid #7f1d1d',
    color: '#fca5a5',
    padding: '8px 12px',
    borderRadius: 6,
    fontSize: 12,
    fontFamily: 'monospace',
    marginTop: 6,
  },
}
