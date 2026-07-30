import React, { useEffect } from 'react'
import { initWS } from './ws/client'
import { useUIStore } from './store/uiStore'
import QueryEditor from './components/QueryEditor'
import ResultGrid from './components/ResultGrid'
import DynamicRenderer from './components/DynamicRenderer'

export default function App() {
  const components    = useUIStore(s => s.components)
  const activePage    = useUIStore(s => s.activePage)
  const setActivePage = useUIStore(s => s.setActivePage)

  useEffect(() => { initWS() }, [])

  // Page components from the agent, sorted stably by id
  const agentPages = [...components.values()]
    .filter(c => c.type === 'page' && c.id !== 'page-editor')
    .sort((a, b) => a.id.localeCompare(b.id))

  const navPages = [
    { id: 'page-editor', label: 'Query Editor' },
    ...agentPages.map(p => ({ id: p.id, label: p.label ?? p.id })),
  ]

  return (
    <div style={s.root}>
      {/* ── Left nav ── */}
      <nav style={s.nav}>
        <div style={s.navHeader}>Pages</div>
        {navPages.map(p => (
          <button
            key={p.id}
            onClick={() => setActivePage(p.id)}
            style={{ ...s.navItem, ...(activePage === p.id ? s.navItemActive : {}) }}
          >
            {p.label}
          </button>
        ))}
      </nav>

      {/* ── Right content ── */}
      <div style={s.content}>
        <div style={s.contentInner}>
          {activePage === 'page-editor' ? (
            <>
              <div style={s.editorWrap}><QueryEditor /></div>
              <div style={s.gridWrap}><ResultGrid /></div>
            </>
          ) : (
            <DynamicRenderer pageId={activePage} />
          )}
        </div>
      </div>
    </div>
  )
}

const s: Record<string, React.CSSProperties> = {
  root: {
    display: 'flex',
    height: '100vh',
    background: '#0f1117',
    color: '#e2e8f0',
    overflow: 'hidden',
  },
  nav: {
    width: 'auto',
    minWidth: 120,
    borderRight: '1px solid #1e2433',
    display: 'flex',
    flexDirection: 'column',
    background: '#0b0f1a',
    flexShrink: 0,
    overflowY: 'auto',
  },
  navHeader: {
    padding: '20px 20px 10px',
    fontSize: 11,
    fontWeight: 700,
    color: '#4a5568',
    textTransform: 'uppercase',
    letterSpacing: 1,
  },
  navItem: {
    textAlign: 'left',
    background: 'transparent',
    border: 'none',
    color: '#94a3b8',
    padding: '10px 20px',
    fontSize: 14,
    cursor: 'pointer',
    borderLeft: '3px solid transparent',
    whiteSpace: 'nowrap',
    width: '100%',
  },
  navItemActive: {
    color: '#a5b4fc',
    borderLeftColor: '#6366f1',
    background: '#141929',
  },
  content: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  },
  contentInner: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
    padding: '16px',
    gap: 12,
    minHeight: 0,
  },
  editorWrap: { flexShrink: 0 },
  gridWrap: {
    flex: 1,
    overflow: 'hidden',
    display: 'flex',
    flexDirection: 'column',
  },
}
