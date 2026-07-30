import React, { useState } from 'react'
import { AgGridReact } from 'ag-grid-react'
import type { ColDef, RowClickedEvent, ICellRendererParams } from 'ag-grid-community'
import 'ag-grid-community/styles/ag-grid.css'
import 'ag-grid-community/styles/ag-theme-quartz.css'
import {
  ResponsiveContainer,
  BarChart, Bar,
  LineChart, Line,
  AreaChart, Area,
  PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, Legend,
} from 'recharts'
import { useUIStore } from '../store/uiStore'
import type { ComponentSpec } from '../store/uiStore'
import { sendAction } from '../ws/client'

const CHART_COLORS = ['#6366f1','#22d3ee','#f59e0b','#34d399','#f87171','#a78bfa','#fb923c','#94a3b8']

// Derive a short action label from a component's label/placeholder
function actionLabel(label: unknown, placeholder: unknown, fallback = 'Apply'): string {
  const text = (String(label ?? '') + ' ' + String(placeholder ?? '')).toLowerCase()
  if (text.includes('search')) return 'Search'
  if (text.includes('filter')) return 'Filter'
  if (text.includes('email') || text.includes('name')) return 'Save'
  return fallback
}

// Shared plain action button used inline next to inputs
function ActionBtn({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button onClick={onClick} style={s.actionBtn}>{label}</button>
  )
}

// ── DataGrid ──────────────────────────────────────────────────────────────────

function ActionsCellRenderer(params: ICellRendererParams & { colDef: any }) {
  const actions: { label: string; navigate?: string }[] = params.colDef.cellRendererParams?.actions ?? []
  const setActivePage = useUIStore(s => s.setActivePage)
  const row = params.data

  return (
    <div style={{ display: 'flex', gap: 4, alignItems: 'center', height: '100%' }}>
      {actions.map(action => (
        <button
          key={action.label}
          onClick={e => {
            e.stopPropagation()
            if (action.navigate) setActivePage(action.navigate)
            sendAction('row_click', {
              component_id: params.colDef.cellRendererParams?.componentId ?? '',
              page_id: params.colDef.cellRendererParams?.pageId ?? '',
              row,
              navigate: action.navigate ?? '',
              action_type: action.label.toLowerCase(),
            })
          }}
          style={{ fontSize: 11, padding: '1px 8px', cursor: 'pointer', background: '#1e2433', border: '1px solid #2d3748', color: '#94a3b8', borderRadius: 3 }}
        >
          {action.label}
        </button>
      ))}
    </div>
  )
}

function DataGrid({ spec }: { spec: ComponentSpec }) {
  const p = spec.props ?? {}
  const columns = (p.columns as any[]) ?? []
  const rows = (p.rows as any[]) ?? []

  const colDefs: ColDef[] = columns.length > 0
    ? columns.map((c: any): ColDef => {
        if (c.type === 'actions') {
          return {
            field: c.field ?? 'actions',
            headerName: c.header ?? 'Actions',
            sortable: false,
            filter: false,
            resizable: false,
            suppressMovable: true,
            pinned: 'right' as const,
            width: c.width ?? 160,
            cellRenderer: ActionsCellRenderer,
            cellRendererParams: {
              actions: c.actions ?? [],
              componentId: spec.id,
              pageId: spec.pageId ?? '',
            },
          }
        }
        return {
          field: c.field ?? c.key,
          headerName: c.header ?? c.headerName ?? c.label ?? c.field ?? c.key,
          sortable: c.sortable !== false,
          filter: c.filterable !== false,
          resizable: true,
          width: c.width,
          pinned: c.pinned,
        }
      })
    : rows.length > 0
      ? Object.keys(rows[0]).map(k => ({ field: k, headerName: k, sortable: true, filter: true, resizable: true }))
      : []

  const handleRowClick = (e: RowClickedEvent) => {
    sendAction('row_click', { component_id: spec.id, page_id: spec.pageId ?? '', row: e.data })
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      {spec.label && <div style={s.sectionLabel}>{spec.label}</div>}
      <div className="ag-theme-quartz-dark" style={{ height: 320, width: '100%' }}>
        <AgGridReact
          columnDefs={colDefs}
          rowData={rows}
          defaultColDef={{ flex: 1, minWidth: 80 }}
          animateRows
          pagination={!!p.pagination}
          paginationPageSize={(p.pagination as any)?.pageSize ?? 20}
          onRowClicked={handleRowClick}
          rowStyle={{ cursor: 'pointer' }}
        />
      </div>
      <div style={s.rowCount}>{rows.length} row{rows.length !== 1 ? 's' : ''}</div>
    </div>
  )
}

// ── TextInput ─────────────────────────────────────────────────────────────────

function TextInput({ spec }: { spec: ComponentSpec }) {
  const p = spec.props ?? {}
  const [val, setVal] = useState(String(p.value ?? ''))
  const Tag = p.multiline ? 'textarea' : 'input'
  const btnLabel = actionLabel(p.label, p.placeholder)

  const submit = () => sendAction('input_submit', {
    component_id: spec.id, page_id: spec.pageId ?? '', value: val,
  })

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      {Boolean(p.label) && <label style={s.label}>{String(p.label)}</label>}
      <div style={{ display: 'flex', gap: 4 }}>
        <Tag
          value={val}
          placeholder={String(p.placeholder ?? '')}
          onChange={(e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => setVal(e.target.value)}
          onKeyDown={(e: React.KeyboardEvent) => { if (e.key === 'Enter' && !p.multiline) submit() }}
          style={{ ...s.input, ...(p.multiline ? { minHeight: 64, resize: 'vertical' } as React.CSSProperties : {}) }}
          rows={p.multiline ? 3 : undefined}
        />
        {!p.multiline && <ActionBtn label={btnLabel} onClick={submit} />}
      </div>
      {Boolean(p.multiline) && <ActionBtn label={btnLabel} onClick={submit} />}
    </div>
  )
}

// ── Button ────────────────────────────────────────────────────────────────────

const variantStyle: Record<string, React.CSSProperties> = {
  primary:   { background: '#2563eb', color: '#fff',     border: '1px solid #1d4ed8' },
  secondary: { background: '#1e2433', color: '#cbd5e1',  border: '1px solid #2d3748' },
  danger:    { background: '#dc2626', color: '#fff',     border: '1px solid #b91c1c' },
  ghost:     { background: 'transparent', color: '#94a3b8', border: '1px solid transparent' },
}

function Button({ spec }: { spec: ComponentSpec }) {
  const p = spec.props ?? {}
  const vs = variantStyle[String(p.variant ?? 'secondary')] ?? variantStyle.secondary
  const handleClick = () => sendAction('button_click', {
    component_id: spec.id, page_id: spec.pageId ?? '', label: p.label ?? spec.label ?? '',
  })
  return (
    <button disabled={!!p.disabled} onClick={handleClick}
      style={{ ...s.btn, ...vs, opacity: p.disabled ? 0.5 : 1 }}>
      {String(p.label ?? spec.label ?? '')}
    </button>
  )
}

// ── Select ────────────────────────────────────────────────────────────────────

function Select({ spec }: { spec: ComponentSpec }) {
  const p = spec.props ?? {}
  // options can be [{label,value}] or plain strings
  const rawOpts = (p.options as any[]) ?? []
  const opts = rawOpts.map((o: any) =>
    typeof o === 'object' && o !== null ? o : { label: String(o), value: String(o) }
  )
  const [val, setVal] = useState(String(p.value ?? ''))

  const submit = () => sendAction('input_submit', {
    component_id: spec.id, page_id: spec.pageId ?? '', value: val,
  })

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      {Boolean(p.label) && <label style={s.label}>{String(p.label)}</label>}
      <div style={{ display: 'flex', gap: 4 }}>
        <select value={val} onChange={e => setVal(e.target.value)} style={s.input}>
          {Boolean(p.placeholder) && <option value="">{String(p.placeholder)}</option>}
          {opts.map((o: any) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
        <ActionBtn label="Apply" onClick={submit} />
      </div>
    </div>
  )
}

// ── DatePicker ────────────────────────────────────────────────────────────────

function DatePicker({ spec }: { spec: ComponentSpec }) {
  const p = spec.props ?? {}
  const isRange = p.mode === 'range' || Boolean(p.range)
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')

  const submit = () => sendAction('input_submit', {
    component_id: spec.id, page_id: spec.pageId ?? '',
    value: isRange ? { from, to } : from,
  })

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      {Boolean(p.label) && <label style={s.label}>{String(p.label)}</label>}
      <div style={{ display: 'flex', gap: 4, alignItems: 'center', flexWrap: 'wrap' }}>
        <input type="date" value={from} onChange={e => setFrom(e.target.value)} style={s.input} />
        {isRange && <>
          <span style={{ color: '#64748b', fontSize: 12 }}>–</span>
          <input type="date" value={to} onChange={e => setTo(e.target.value)} style={s.input} />
        </>}
        <ActionBtn label="Apply" onClick={submit} />
      </div>
    </div>
  )
}

// ── BadgeFilter ───────────────────────────────────────────────────────────────

function BadgeFilter({ spec }: { spec: ComponentSpec }) {
  const p = spec.props ?? {}
  const opts = (p.options as { label: string; value: string }[]) ?? []
  const [active, setActive] = useState<Set<string>>(new Set())

  const toggle = (v: string) => {
    setActive(prev => {
      const next = new Set(prev)
      if (next.has(v)) next.delete(v); else next.add(v)
      return next
    })
  }

  const submit = () => sendAction('input_submit', {
    component_id: spec.id, page_id: spec.pageId ?? '', value: [...active],
  })

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      {Boolean(p.label) && <label style={s.label}>{String(p.label)}</label>}
      <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap', alignItems: 'center' }}>
        {opts.map(o => (
          <button key={o.value} onClick={() => toggle(o.value)} style={{
            ...s.btn,
            background: active.has(o.value) ? '#2563eb' : '#1e2433',
            color: active.has(o.value) ? '#fff' : '#94a3b8',
            border: `1px solid ${active.has(o.value) ? '#1d4ed8' : '#2d3748'}`,
            borderRadius: 10, padding: '2px 10px', fontSize: 12,
          }}>{o.label}</button>
        ))}
        <ActionBtn label="Filter" onClick={submit} />
      </div>
    </div>
  )
}

// ── Chart ─────────────────────────────────────────────────────────────────────

function Chart({ spec }: { spec: ComponentSpec }) {
  const p = spec.props ?? {}
  const chartType = String(p.chartType ?? 'bar')
  const height = Number(p.height ?? 280)

  // Normalise data: accept rows[] or Chart.js-style {labels, datasets}
  let rows: any[] = []
  if (Array.isArray(p.rows)) {
    rows = p.rows as any[]
  } else if (p.data && typeof p.data === 'object') {
    const d = p.data as any
    if (Array.isArray(d.labels) && Array.isArray(d.datasets?.[0]?.data)) {
      rows = d.labels.map((label: string, i: number) => ({
        name: label,
        value: d.datasets[0].data[i],
      }))
    }
  }

  const xField: string = (p.xAxis as any)?.field ?? 'name'
  const rawYAxes = (p.yAxis as any[]) ?? []
  const yAxes = rawYAxes.length > 0
    ? rawYAxes
    : [{ field: Object.keys(rows[0] ?? {}).find(k => k !== xField) ?? 'value' }]

  const tickStyle = { fontSize: 11, fill: '#94a3b8' }
  const gridStyle = { strokeDasharray: '3 3', stroke: '#2d3748' }
  const margin = { top: 5, right: 16, bottom: 5, left: 0 }

  const wrap = (chart: React.ReactNode) => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      {spec.label && <div style={s.sectionLabel}>{spec.label}</div>}
      <ResponsiveContainer width="100%" height={height}>{chart as any}</ResponsiveContainer>
    </div>
  )

  if (chartType === 'pie' || chartType === 'donut') {
    const dataKey = yAxes[0]?.field ?? 'value'
    return wrap(
      <PieChart>
        <Pie
          data={rows} dataKey={dataKey} nameKey={xField}
          innerRadius={chartType === 'donut' ? '45%' : 0}
          outerRadius="70%" label={({ name, percent }: any) => `${name} ${(percent * 100).toFixed(0)}%`}
          labelLine={false}
        >
          {rows.map((_: any, i: number) => <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />)}
        </Pie>
        <Tooltip />
        <Legend />
      </PieChart>
    )
  }

  if (chartType === 'line') {
    return wrap(
      <LineChart data={rows} margin={margin}>
        <CartesianGrid {...gridStyle} />
        <XAxis dataKey={xField} tick={tickStyle} />
        <YAxis tick={tickStyle} width={40} />
        <Tooltip />
        {Boolean(p.legend) && <Legend />}
        {yAxes.map((y: any, i: number) => (
          <Line key={y.field} type="monotone" dataKey={y.field}
            stroke={CHART_COLORS[i % CHART_COLORS.length]} strokeWidth={2} dot={false} />
        ))}
      </LineChart>
    )
  }

  if (chartType === 'area') {
    return wrap(
      <AreaChart data={rows} margin={margin}>
        <CartesianGrid {...gridStyle} />
        <XAxis dataKey={xField} tick={tickStyle} />
        <YAxis tick={tickStyle} width={40} />
        <Tooltip />
        {yAxes.map((y: any, i: number) => (
          <Area key={y.field} type="monotone" dataKey={y.field}
            fill={CHART_COLORS[i % CHART_COLORS.length] + '33'}
            stroke={CHART_COLORS[i % CHART_COLORS.length]} />
        ))}
      </AreaChart>
    )
  }

  // Default: bar
  return wrap(
    <BarChart data={rows} margin={margin}>
      <CartesianGrid {...gridStyle} />
      <XAxis dataKey={xField} tick={tickStyle} />
      <YAxis tick={tickStyle} width={40} />
      <Tooltip />
      {Boolean(p.legend) && <Legend />}
      {yAxes.map((y: any, i: number) => (
        <Bar key={y.field} dataKey={y.field}
          fill={CHART_COLORS[i % CHART_COLORS.length]} radius={[3, 3, 0, 0]} />
      ))}
    </BarChart>
  )
}

// ── KpiCard ───────────────────────────────────────────────────────────────────

function KpiCard({ spec }: { spec: ComponentSpec }) {
  const p = spec.props ?? {}
  const trend = p.trend as { direction: string; value: string; label: string } | null | undefined
  const trendColor = trend?.direction === 'up' ? '#34d399' : trend?.direction === 'down' ? '#f87171' : '#94a3b8'
  return (
    <div style={{ background: '#1e2433', border: '1px solid #2d3748', borderRadius: 6, padding: '10px 16px', minWidth: 120 }}>
      <div style={{ fontSize: 11, color: '#64748b', textTransform: 'uppercase', letterSpacing: 1, marginBottom: 4 }}>
        {String(p.label ?? spec.label ?? '')}
      </div>
      <div style={{ fontSize: 22, fontWeight: 700, color: '#e2e8f0' }}>
        {String(p.value ?? '—')}
        {p.unit != null && <span style={{ fontSize: 13, color: '#64748b', fontWeight: 400 }}> {String(p.unit)}</span>}
      </div>
      {trend && <div style={{ fontSize: 11, marginTop: 2, color: trendColor }}>{trend.value} {trend.label}</div>}
    </div>
  )
}

// ── Toolbar ───────────────────────────────────────────────────────────────────

function Toolbar({ spec }: { spec: ComponentSpec }) {
  const components = useUIStore(st => st.components)
  const p = spec.props ?? {}
  const children = (p.children as string[]) ?? []
  return (
    <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', padding: '4px 0' }}>
      {children.map(id => {
        const child = components.get(id)
        return child ? <ComponentNode key={id} spec={child} /> : null
      })}
    </div>
  )
}

// ── DetailPanel ───────────────────────────────────────────────────────────────

function DetailPanel({ spec }: { spec: ComponentSpec }) {
  const p = spec.props ?? {}
  const fields = (p.fields as any[]) ?? []
  return (
    <div style={{ border: '1px solid #2d3748', padding: '10px 14px', fontSize: 13 }}>
      <div style={{ fontWeight: 600, color: '#cbd5e1', marginBottom: 8 }}>
        {String(p.title ?? spec.label ?? 'Detail')}
      </div>
      {fields.length > 0
        ? fields.map((f: any) => (
            <div key={f.key ?? f.field} style={{ display: 'flex', gap: 8, marginBottom: 6, alignItems: 'center' }}>
              <span style={{ color: '#64748b', minWidth: 100, fontSize: 12 }}>{f.label ?? f.key}</span>
              {f.editable
                ? <input defaultValue={String(f.value ?? '')} style={{ ...s.input, flex: 1 }} />
                : <span style={{ color: '#e2e8f0' }}>{String(f.value ?? '')}</span>
              }
            </div>
          ))
        : <div style={{ color: '#4a5568', fontStyle: 'italic' }}>Select a row to view details</div>
      }
    </div>
  )
}

// ── ComponentNode (dispatcher) ────────────────────────────────────────────────

function ComponentNode({ spec }: { spec: ComponentSpec }) {
  switch (spec.type) {
    case 'data-grid':    return <DataGrid spec={spec} />
    case 'text-input':   return <TextInput spec={spec} />
    case 'button':       return <Button spec={spec} />
    case 'select':       return <Select spec={spec} />
    case 'date-picker':  return <DatePicker spec={spec} />
    case 'badge-filter': return <BadgeFilter spec={spec} />
    case 'kpi-card':     return <KpiCard spec={spec} />
    case 'chart':        return <Chart spec={spec} />
    case 'toolbar':      return <Toolbar spec={spec} />
    case 'detail-panel': return <DetailPanel spec={spec} />
    case 'notification': {
      const p = spec.props ?? {}
      return (
        <div style={{ border: '1px solid #2d3748', padding: '6px 12px', fontSize: 13, color: '#e2e8f0' }}>
          {String(p.message ?? '')}
        </div>
      )
    }
    case 'divider':
      return <hr style={{ border: 'none', borderTop: '1px solid #2d3748', margin: '6px 0' }} />
    default:
      return (
        <div style={{ border: '1px solid #2d3748', padding: '6px 10px', fontSize: 12, color: '#64748b' }}>
          [{spec.type}] {spec.label ?? spec.id}
        </div>
      )
  }
}

// ── Zone layout ───────────────────────────────────────────────────────────────

export default function DynamicRenderer({ pageId }: { pageId: string }) {
  const components = useUIStore(st => st.components)

  const pageComponents = [...components.values()].filter(
    c => c.type !== 'page' && c.pageId === pageId
  )

  const byZone = (zone: string) =>
    pageComponents
      .filter(c => c.position?.zone === zone || (!c.position && zone === 'main'))
      .sort((a, b) => (a.position?.order ?? 0) - (b.position?.order ?? 0))

  if (pageComponents.length === 0) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#2d3748', fontSize: 13, fontStyle: 'italic' }}>
        No components yet — run a query to let the agent build this page
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8, padding: '8px 0', overflowY: 'auto', flex: 1 }}>
      {byZone('toolbar').length > 0 && (
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          {byZone('toolbar').map(c => <ComponentNode key={c.id} spec={c} />)}
        </div>
      )}
      <div style={{ display: 'flex', gap: 12 }}>
        {byZone('sidebar').length > 0 && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8, minWidth: 180, maxWidth: 260 }}>
            {byZone('sidebar').map(c => <ComponentNode key={c.id} spec={c} />)}
          </div>
        )}
        {byZone('main').length > 0 && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8, flex: 1, minWidth: 0 }}>
            {byZone('main').map(c => <ComponentNode key={c.id} spec={c} />)}
          </div>
        )}
      </div>
      {byZone('footer').length > 0 && (
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          {byZone('footer').map(c => <ComponentNode key={c.id} spec={c} />)}
        </div>
      )}
    </div>
  )
}

// ── Shared styles ─────────────────────────────────────────────────────────────

const s: Record<string, React.CSSProperties> = {
  label:        { fontSize: 11, color: '#94a3b8' },
  sectionLabel: { fontSize: 12, fontWeight: 600, color: '#94a3b8' },
  rowCount:     { fontSize: 11, color: '#4a5568', textAlign: 'right' },
  input: {
    background: '#0f1117',
    border: '1px solid #2d3748',
    color: '#e2e8f0',
    fontSize: 13,
    padding: '4px 8px',
    outline: 'none',
    flex: 1,
    boxSizing: 'border-box' as const,
  },
  btn: {
    padding: '4px 12px',
    fontSize: 13,
    cursor: 'pointer',
    border: '1px solid #2d3748',
    background: '#1e2433',
    color: '#cbd5e1',
    borderRadius: 3,
  },
  actionBtn: {
    padding: '4px 10px',
    fontSize: 12,
    cursor: 'pointer',
    border: '1px solid #4a5568',
    background: '#1e2433',
    color: '#94a3b8',
    borderRadius: 3,
    whiteSpace: 'nowrap' as const,
  },
}
