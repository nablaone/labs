import React, { useMemo } from 'react'
import { AgGridReact } from 'ag-grid-react'
import type { ColDef } from 'ag-grid-community'
import 'ag-grid-community/styles/ag-grid.css'
import 'ag-grid-community/styles/ag-theme-quartz.css'
import { useQueryStore } from '../store/queryStore'

export default function ResultGrid() {
  const { result, loading } = useQueryStore()

  const columnDefs = useMemo<ColDef[]>(() => {
    if (!result?.columns?.length) return []
    return result.columns.map((col) => ({
      field: col,
      headerName: col,
      sortable: true,
      filter: true,
      resizable: true,
      minWidth: 80,
    }))
  }, [result?.columns])

  if (!result && !loading) {
    return (
      <div style={styles.empty}>
        Run a query to see results
      </div>
    )
  }

  if (loading) {
    return <div style={styles.empty}>Running…</div>
  }

  if (result?.error) {
    return null // error shown in editor
  }

  const rowCount = result?.rows?.length ?? 0

  return (
    <div style={styles.wrap}>
      <div style={styles.statusBar}>
        {rowCount} row{rowCount !== 1 ? 's' : ''}
      </div>
      <div className="ag-theme-quartz-dark" style={styles.grid}>
        <AgGridReact
          columnDefs={columnDefs}
          rowData={result?.rows ?? []}
          defaultColDef={{ flex: 1, minWidth: 80 }}
          suppressMovableColumns={false}
          animateRows={true}
        />
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  wrap: { display: 'flex', flexDirection: 'column', flex: 1, overflow: 'hidden', gap: 0 },
  statusBar: {
    fontSize: 11,
    color: '#4a5568',
    padding: '4px 2px',
    textAlign: 'right',
  },
  grid: { flex: 1, width: '100%' },
  empty: {
    flex: 1,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    color: '#2d3748',
    fontSize: 15,
    fontStyle: 'italic',
  },
}
