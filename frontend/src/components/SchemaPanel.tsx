import React, { useEffect, useState } from 'react'
import { useQueryStore } from '../store/queryStore'

interface ColumnInfo {
  name: string
  data_type: string
  nullable: boolean
  is_primary: boolean
  foreign_key: string
}

interface TableSchema {
  table: string
  columns: ColumnInfo[]
  error?: string
}

export default function SchemaPanel() {
  const [tables, setTables] = useState<string[]>([])
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [schemas, setSchemas] = useState<Record<string, TableSchema>>({})
  const setSql = useQueryStore((s) => s.setSql)

  useEffect(() => {
    fetch('/schema')
      .then((r) => r.json())
      .then(setTables)
      .catch(() => {})
  }, [])

  const toggleTable = async (table: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(table)) {
        next.delete(table)
      } else {
        next.add(table)
      }
      return next
    })
    if (!schemas[table]) {
      const res = await fetch(`/schema/${table}`)
      const data = await res.json()
      setSchemas((prev) => ({ ...prev, [table]: data }))
    }
  }

  return (
    <div style={styles.panel}>
      <div style={styles.title}>Schema</div>
      {tables.map((t) => (
        <div key={t}>
          <div
            style={styles.tableRow}
            onClick={() => toggleTable(t)}
            onDoubleClick={() => setSql(`SELECT * FROM ${t}`)}
            title="Double-click to select all"
          >
            <span style={styles.arrow}>{expanded.has(t) ? '▾' : '▸'}</span>
            <span style={styles.tableName}>{t}</span>
          </div>
          {expanded.has(t) && schemas[t]?.columns && (
            <div style={styles.columnList}>
              {schemas[t].columns.map((col) => (
                <div
                  key={col.name}
                  style={styles.colRow}
                  title={[col.data_type, col.nullable ? 'nullable' : 'not null', col.foreign_key ? `FK → ${col.foreign_key}` : ''].filter(Boolean).join(', ')}
                >
                  <span style={styles.colName}>
                    {col.is_primary && <span style={styles.pkBadge}>PK</span>}
                    {col.foreign_key && <span style={styles.fkBadge}>FK</span>}
                    {col.name}
                  </span>
                  <span style={styles.colType}>{col.data_type}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      ))}
      {tables.length === 0 && (
        <div style={styles.empty}>No tables found</div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  panel: {
    width: 220,
    background: '#131720',
    borderRight: '1px solid #1e2433',
    overflowY: 'auto',
    flexShrink: 0,
  },
  title: {
    padding: '10px 12px',
    fontSize: 11,
    fontWeight: 700,
    color: '#4a5568',
    textTransform: 'uppercase',
    letterSpacing: 1,
    borderBottom: '1px solid #1e2433',
  },
  tableRow: {
    display: 'flex',
    alignItems: 'center',
    padding: '7px 10px',
    cursor: 'pointer',
    gap: 6,
    borderBottom: '1px solid #1e2433',
  },
  arrow: { color: '#4a5568', fontSize: 11, width: 10 },
  tableName: { fontSize: 13, color: '#a5b4fc', fontWeight: 500 },
  columnList: { background: '#0d1017' },
  colRow: {
    display: 'flex',
    justifyContent: 'space-between',
    padding: '4px 10px 4px 22px',
    fontSize: 11,
    borderBottom: '1px solid #1a2030',
    cursor: 'default',
  },
  colName: { color: '#cbd5e1', display: 'flex', alignItems: 'center', gap: 4 },
  colType: { color: '#4a5568', fontFamily: 'monospace', fontSize: 10 },
  pkBadge: {
    background: '#1e3a5f',
    color: '#60a5fa',
    padding: '1px 4px',
    borderRadius: 3,
    fontSize: 9,
    fontWeight: 700,
  },
  fkBadge: {
    background: '#2d1b4e',
    color: '#c084fc',
    padding: '1px 4px',
    borderRadius: 3,
    fontSize: 9,
    fontWeight: 700,
  },
  empty: { padding: 12, color: '#2d3748', fontSize: 12 },
}
