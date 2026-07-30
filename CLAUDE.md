# CLAUDE.md — Adaptive Query UI

## Project Overview

An adaptive GUI application that observes user behavior (SQL queries, clicks, interactions) and autonomously builds and evolves its own interface. The system uses an agentic LLM loop running in the background to analyze user intent and emit UI mutations in real time via WebSocket.

The core idea: the user starts with a blank text input and a data grid. Everything else — filters, charts, buttons, forms, detail panels — is discovered and constructed by the agent.

---

## Architecture

```
User Action (query / click / selection)
        │
        ▼
  Go Backend
  ├── Execute SQL → PostgreSQL
  ├── Append to ActionLog
  └── Spawn Agent (async goroutine)
               │
               ▼
         Agent Loop (Claude Sonnet via Anthropic API)
         ├── get_action_history()
         ├── get_current_ui()
         ├── inspect_schema(table)
         ├── run_query(sql)
         ├── add_component(spec)
         ├── update_component(id, props)
         ├── remove_component(id)
         └── bind_action(component_id, trigger, sql_template)
               │
               ▼ (each UI mutation emitted immediately)
         WebSocket broadcast → React frontend
               │
               ▼
         UI patch applied to reactive component tree
```

---

## Stack

| Layer | Technology |
|---|---|
| Backend | Go (1.22+) |
| Database | PostgreSQL via `pgx/v5` |
| WebSocket | `gorilla/websocket` |
| LLM | Claude Sonnet (`claude-sonnet-4-20250514`) via Anthropic Go SDK |
| Frontend | React + TypeScript |
| Data Grid | AG Grid Community |
| Charts | Recharts |
| UI State | Zustand |
| Styling | Tailwind CSS |

---

## Repository Structure

```
/
├── cmd/
│   └── server/           # main entry point
├── internal/
│   ├── db/               # pgx pool, query executor, schema inspector
│   ├── agent/            # agent loop, tool registry, tool handlers
│   ├── uistate/          # UI tree, diff/patch engine
│   ├── actionlog/        # append-only log of user actions
│   └── ws/               # WebSocket hub, broadcast
├── api/
│   ├── rest/             # POST /query, GET /schema
│   └── ws/               # /ws endpoint
├── frontend/
│   ├── src/
│   │   ├── components/   # Grid, Chart, Button, DetailPanel, etc.
│   │   ├── store/        # Zustand UI state
│   │   ├── ws/           # WebSocket client + patch applier
│   │   └── App.tsx
│   └── package.json
├── CLAUDE.md
└── docker-compose.yml
```

---

## Agent Design

### Agent Tools

The agent has exactly these tools available. No direct UI construction happens outside the agent.

```go
// Schema & data inspection
inspect_schema(table string) → SchemaInfo
run_query(sql string) → QueryResult  // max 50 rows, read-only

// Context tools
get_action_history() → []ActionEvent  // last N user actions
get_current_ui() → UITree            // current component tree

// UI mutation tools (each triggers immediate WS broadcast)
add_component(spec ComponentSpec) → string  // returns new component ID
update_component(id string, props map[string]any) → error
remove_component(id string) → error
bind_action(componentID string, trigger string, handler ActionHandler) → error
```

### Agent Loop (Go)

```go
func (a *Agent) Run(ctx context.Context, trigger ActionEvent) error {
    messages := a.buildInitialMessages(trigger)

    for {
        resp, err := a.llm.Complete(ctx, messages, a.tools.Definitions())
        if err != nil { return err }
        if resp.StopReason == "end_turn" { break }

        for _, toolCall := range resp.ToolCalls {
            result := a.tools.Execute(ctx, toolCall)

            // UI mutations broadcast immediately over WebSocket
            if isUIMutation(toolCall.Name) {
                a.wsBroadcast(toUIEvent(toolCall, result))
            }

            messages = append(messages, assistantToolUseMessage(toolCall))
            messages = append(messages, toolResultMessage(toolCall.ID, result))
        }
    }
    return nil
}
```

### Agent Trigger Policy

- Agent runs **asynchronously** — user is never blocked waiting for it
- One agent run per user action; runs are debounced (300ms) to avoid thrashing
- Agent runs are **cancellable**: new user action cancels the previous in-flight run
- Agent never touches the DB directly — all queries go through the same executor as the user (with read-only enforcement for `run_query`)

---

## Component Library

The agent may only instantiate components from this library. No ad-hoc or undocumented component types are allowed. Each entry defines the `type` string, its props contract, supported triggers, and the frontend implementation target.

---

### 1. `data-grid`

A tabular view of query results. The primary output surface.

```typescript
interface DataGridProps {
  columns: {
    field: string           // maps to SQL column name
    header: string          // display label
    type: "text" | "number" | "date" | "datetime" | "boolean" | "badge" | "currency" | "link"
    editable?: boolean      // enables inline cell editing
    sortable?: boolean      // default true
    filterable?: boolean    // default true
    width?: number          // pixels
    pinned?: "left" | "right"
  }[]
  rowSelection?: "single" | "multi" | false
  bulkActions?: boolean     // shows checkbox column + bulk toolbar
  pagination?: { pageSize: number }
  dataSource: string        // component ID of the query that feeds this grid
}
```

Triggers: `row-click`, `row-select`, `cell-edit`, `page-change`
Implementation: AG Grid Community

---

### 2. `text-input`

Single-line or multi-line text field. Used for search, filter, or SQL input.

```typescript
interface TextInputProps {
  label?: string
  placeholder?: string
  multiline?: boolean       // renders as textarea
  debounceMs?: number       // default 300, for live-search
  value?: string            // initial value
  validation?: "none" | "required" | "email" | "number" | "regex"
  validationPattern?: string
}
```

Triggers: `change`, `submit` (Enter key), `blur`

---

### 3. `button`

Action trigger. Can be primary, secondary, danger, or ghost style.

```typescript
interface ButtonProps {
  label: string
  icon?: string             // icon name from Lucide icon set
  variant?: "primary" | "secondary" | "danger" | "ghost"
  size?: "sm" | "md" | "lg"
  disabled?: boolean
  confirmDialog?: {         // shows confirm before firing action
    title: string
    message: string
  }
}
```

Triggers: `click`

---

### 4. `select`

Dropdown for single-value selection. Used for filters, enum fields, FK lookups.

```typescript
interface SelectProps {
  label?: string
  placeholder?: string
  options?:
    | { label: string; value: string }[]           // static list
    | { dynamic: true; sql: string }               // populated at runtime
  multi?: boolean           // multi-select with chips
  clearable?: boolean       // shows ✕ to reset
  value?: string | string[]
}
```

Triggers: `change`

---

### 5. `date-picker`

Date or date-range selector. Used for temporal filters.

```typescript
interface DatePickerProps {
  label?: string
  mode?: "single" | "range"
  value?: string | { from: string; to: string }   // ISO 8601
  minDate?: string
  maxDate?: string
  presets?: ("today" | "yesterday" | "last7d" | "last30d" | "thisMonth" | "thisYear")[]
}
```

Triggers: `change`

---

### 6. `kpi-card`

Single metric display. Used for aggregate query results (COUNT, SUM, AVG).

```typescript
interface KpiCardProps {
  label: string
  value: string | number
  unit?: string             // e.g. "orders", "PLN", "%"
  trend?: {
    direction: "up" | "down" | "flat"
    value: string           // e.g. "+12%"
    label: string           // e.g. "vs last month"
  }
  color?: "default" | "green" | "red" | "yellow" | "blue"
  dataSource?: string       // component ID; auto-updates when source refreshes
}
```

Triggers: `click` (optional drill-down)

---

### 7. `chart`

Data visualization bound to a query result.

```typescript
interface ChartProps {
  chartType: "bar" | "line" | "area" | "pie" | "donut" | "scatter"
  dataSource: string        // component ID of the data-grid or query driving this chart
  xAxis: { field: string; label?: string }
  yAxis: { field: string; label?: string }[]  // multiple series supported
  colorScheme?: "default" | "categorical" | "sequential"
  legend?: boolean
  stacked?: boolean
  height?: number           // pixels, default 300
}
```

Triggers: `segment-click` (filters bound grid to clicked value)

---

### 8. `detail-panel`

Side panel showing all fields of a single selected row. Appears when user clicks a grid row.

```typescript
interface DetailPanelProps {
  title?: string            // defaults to table name + row ID
  sourceGrid: string        // component ID of the grid driving this panel
  fields?: {
    field: string
    label: string
    type: "text" | "number" | "date" | "datetime" | "boolean" | "badge" | "link"
    editable?: boolean
  }[]                       // if omitted, all columns are shown
  actions?: string[]        // component IDs of buttons to embed in panel header
}
```

Triggers: `field-edit`, `close`

---

### 9. `modal`

Dialog for insert, edit, or delete operations. Contains a form.

```typescript
interface ModalProps {
  title: string
  mode: "insert" | "edit" | "delete" | "confirm" | "custom"
  targetTable?: string      // for insert/edit/delete
  fields?: {
    field: string
    label: string
    type: "text" | "number" | "date" | "datetime" | "boolean" | "select" | "textarea"
    required?: boolean
    defaultValue?: any
    options?: { label: string; value: string }[]   // for select fields
    placeholder?: string
  }[]
  confirmMessage?: string   // for mode: "confirm" or "delete"
  submitLabel?: string      // default: "Save" / "Delete" / "Confirm"
  cancelLabel?: string      // default: "Cancel"
}
```

Triggers: `submit`, `cancel`

---

### 10. `toolbar`

Horizontal strip grouping buttons and controls into a named action zone.

```typescript
interface ToolbarProps {
  children: string[]        // ordered list of component IDs to render inside
  align?: "left" | "right" | "space-between"
  bordered?: boolean
}
```

No direct triggers; acts as a layout container.

---

### 11. `tabs`

Tabbed container for switching between multiple views or grids.

```typescript
interface TabsProps {
  tabs: {
    id: string
    label: string
    icon?: string
    children: string[]      // component IDs shown in this tab
  }[]
  defaultTab?: string
}
```

Triggers: `tab-change`

---

### 12. `badge-filter`

Horizontal list of toggleable badge buttons. Used for quick enum/status filtering.

```typescript
interface BadgeFilterProps {
  label?: string
  options: { label: string; value: string; color?: string }[]
  multi?: boolean
  value?: string[]
}
```

Triggers: `change`

---

### 13. `pagination`

Standalone pagination control. Used when grid pagination is managed externally.

```typescript
interface PaginationProps {
  total: number             // total row count
  pageSize: number
  currentPage: number
  pageSizeOptions?: number[]
  boundTo: string           // component ID of the grid to control
}
```

Triggers: `page-change`, `page-size-change`

---

### 14. `notification`

Transient toast/snackbar notification. Shown after mutations (insert, update, delete).

```typescript
interface NotificationProps {
  message: string
  severity: "info" | "success" | "warning" | "error"
  durationMs?: number       // default 4000; 0 = persistent
  action?: { label: string; componentId: string }  // optional undo button
}
```

No triggers. Auto-dismissed after `durationMs`.

---

### 15. `divider`

Visual separator between layout zones or component groups.

```typescript
interface DividerProps {
  orientation?: "horizontal" | "vertical"
  label?: string            // optional section label rendered in divider
}
```

No triggers.

---

### Component Type Reference (quick lookup for agent)

| Type | Use when |
|---|---|
| `data-grid` | Any `SELECT` returning tabular rows |
| `text-input` | Free-text search, SQL editor, form fields |
| `button` | Any user-triggered action (insert, delete, export, run) |
| `select` | Enum column filter, FK lookup, category picker |
| `date-picker` | Column named `*_at`, `*_date`, or `date_trunc` in query |
| `kpi-card` | `COUNT`, `SUM`, `AVG`, `MIN`, `MAX` in SELECT list |
| `chart` | `GROUP BY` with numeric aggregate; time series data |
| `detail-panel` | User clicked a grid row; show full record |
| `modal` | Insert new record, edit existing record, confirm delete |
| `toolbar` | Grouping action buttons above or below a grid |
| `tabs` | Multiple grids or views the user navigates between |
| `badge-filter` | Status/type enum with ≤ 8 distinct values |
| `pagination` | Large result sets (> 100 rows) |
| `notification` | After any write operation completes |
| `divider` | Separating logical sections in a dense layout |

---

### Layout Zones

All components are placed into one of these zones:

```
┌─────────────────────────────────────────────┐
│  toolbar (buttons, filters, search)         │
├──────────────┬──────────────────────────────┤
│              │                              │
│   sidebar    │         main                 │
│  (filters,   │   (grid, chart, kpi-cards)   │
│   detail     │                              │
│   panel)     │                              │
│              │                              │
├──────────────┴──────────────────────────────┤
│  footer (pagination, status, bulk actions)  │
└─────────────────────────────────────────────┘
```

`position.zone` must be one of: `"toolbar"`, `"sidebar"`, `"main"`, `"footer"`.
`position.order` is an integer; lower values appear first within a zone.

---

## UI State Protocol

### UIEvent (WebSocket message)

```typescript
type UIEvent =
  | { op: "add";    component: ComponentSpec }
  | { op: "update"; id: string; props: Partial<ComponentSpec> }
  | { op: "remove"; id: string }
  | { op: "bind";   componentId: string; trigger: string; handler: ActionHandler }
```

### ComponentSpec

```typescript
interface ComponentSpec {
  id: string
  type:
    | "data-grid"
    | "text-input"
    | "button"
    | "select"
    | "date-picker"
    | "kpi-card"
    | "chart"
    | "detail-panel"
    | "modal"
    | "toolbar"
    | "tabs"
    | "badge-filter"
    | "pagination"
    | "notification"
    | "divider"
  label?: string
  props?: Record<string, any>   // typed per component — see Component Library
  position?: { zone: "toolbar" | "sidebar" | "main" | "footer"; order: number }
}
```

### ActionHandler

```typescript
interface ActionHandler {
  type: "query"   // execute SQL template, refresh bound component
       | "modal"  // open insert/edit/delete modal
       | "nav"    // drill-down to related entity
       | "filter" // apply filter to grid
  payload: string  // SQL template with :param placeholders, or component ID
}
```

---

## Query Intent Classification

The agent must classify every SQL query into one of two intents before deciding what UI to build. The intent drives the entire response — different queries warrant entirely different component sets.

---

### CRUD Queries

**Definition**: Queries that operate on individual rows of a single entity — reading, creating, editing, or deleting records.

**Signals**:
- `SELECT * FROM <table>` or `SELECT <cols> FROM <table>` with no aggregates
- `SELECT ... WHERE id = $1` (lookup by primary key)
- `INSERT INTO <table>`, `UPDATE <table>`, `DELETE FROM <table>`
- `SELECT` returning all columns of a single table (even with simple `WHERE` filters)

**UI response** — the agent should build a full CRUD surface:

| Component | Purpose |
|---|---|
| `data-grid` in `main` | Displays all rows; rows are clickable |
| `button` ("New …") in `toolbar` | Navigates to a create-form page |
| Sidebar filters (`select`, `date-picker`, `text-input`, `badge-filter`) | Filter the grid |
| Row click → navigate to detail/edit page | Show all fields with an Edit button |
| Edit page: form inputs + Save/Cancel buttons | Navigate back on save |
| Delete button (on detail page or grid toolbar) | Confirm then delete |

**Never add** KPI cards, charts, or breakdown grids for CRUD queries.

---

### Analytical / Report Queries

**Definition**: Queries that aggregate, group, or join data to answer a business question. The result is a summary, not a list of records to manage.

**Signals**:
- `GROUP BY` clause present
- Aggregate functions: `COUNT(*)`, `SUM(...)`, `AVG(...)`, `MIN(...)`, `MAX(...)`
- `date_trunc(...)` in `SELECT` or `GROUP BY`
- Multi-table `JOIN` with aggregation
- `HAVING` clause
- Subqueries or CTEs computing metrics

**UI response** — the agent should build a read-only analytics surface:

| Component | Purpose |
|---|---|
| `kpi-card` (one per scalar metric) in `toolbar` | Shows COUNT, SUM, AVG values |
| `chart` in `main` | Visual summary — type depends on data shape (see below) |
| `data-grid` in `main` (below the chart) | Full breakdown table |

**Never add** New/Edit/Delete buttons, form pages, or row-action navigation for analytical queries.

#### Chart Type Selection

Pick the chart type based on what the data is showing:

| Data pattern | Chart type | Rationale |
|---|---|---|
| GROUP BY a timestamp / `date_trunc(...)` column | `line` | Time series — shows trend |
| GROUP BY a timestamp with cumulative/running total | `area` | Volume over time |
| GROUP BY an enum/status/category, ≤ 8 groups, values sum to a meaningful whole | `pie` | Part-to-whole comparison |
| GROUP BY an enum/category, > 8 groups, or comparison is the goal | `bar` | Side-by-side comparison |
| GROUP BY two categorical columns (matrix) | `bar` (stacked) | Composition by category |
| Two numeric columns, no GROUP BY | `scatter` | Correlation |

**Decision shortcuts:**
- "How is X distributed across statuses/types?" → `pie` (if ≤ 8 slices)
- "How did X change over time?" → `line`
- "Which category has the most X?" → `bar`
- "What's the volume of X over time?" → `area`

**Data format** the agent must include in `chart` props:
```json
{
  "chartType": "pie",
  "rows": [{"status": "shipped", "count": 4}, {"status": "pending", "count": 3}],
  "xAxis": {"field": "status", "label": "Status"},
  "yAxis": [{"field": "count", "label": "Orders"}],
  "height": 280
}
```
`rows` carries the actual data inline (same pattern as `data-grid`). `xAxis.field` is the category/time axis; `yAxis[].field` are the numeric series.

---

### Classification Rules

1. If the query has **any aggregate function or GROUP BY** → Analytical.
2. If the query is a plain `SELECT` on one table with no aggregates → CRUD.
3. `INSERT` / `UPDATE` / `DELETE` → CRUD (build or update the relevant form page).
4. When in doubt, inspect the result shape: many identical-looking columns with one row per entity → CRUD; few columns with counts/sums → Analytical.
5. A page should never mix CRUD controls (New/Edit/Delete buttons) with Analytical components (KPI cards). If the user writes both query types for the same table, maintain two separate pages.

---

## Schema Cache (`data/schema.json`)

The backend analyzes the database schema on startup and persists business-logic annotations to `data/schema.json`. Schema changes are detected via SHA1 of `information_schema.columns`; if the hash changes, the cache is regenerated with a fresh LLM call.

### `ColumnMeta` fields

| Field | Type | Purpose |
|---|---|---|
| `name` | string | Raw column name |
| `business_name` | string | Human-readable label (used as column header / form label) |
| `hide_in_ui` | bool | Omit from grids and forms (PKs and raw FK IDs) |
| `display_hint` | string | Rendering hint for the frontend |
| `fk_ref` | string | For FK cols: "table.display_column" (e.g. "customers.name") |

### `display_hint` values

| Hint | Meaning | Grid column type |
|---|---|---|
| `pk` | Primary key — always hidden | — |
| `fk` | Foreign key integer — hidden; JOIN to display value | — |
| `badge` | Enum/status field | `"badge"` |
| `currency` | Monetary amount | `"currency"` |
| `datetime` | Timestamp | `"datetime"` |
| `date` | Date-only | `"date"` |
| `email` | Email address | `"link"` |
| `text` | Plain string | `"text"` |
| `number` | Numeric (non-monetary) | `"number"` |

### Agent behavior rules

1. **Always use `business_name`** as the column header — never the raw `name`.
2. **Skip `hide_in_ui: true` columns** from every grid and form.
3. **FK columns** (`display_hint: "fk"`) must never appear as raw integers. Rewrite the query to LEFT JOIN the referenced table (from `fk_ref`) and select the display column instead. Name the selected alias using the referenced table's natural name (e.g., `c.name AS customer_name`).
4. **Map `display_hint` → grid column type** directly (badge, currency, datetime, date, link, text, number).

---

## UI Heuristics the Agent Should Apply

These are guidelines embedded in the agent system prompt — not hardcoded rules.

| Observed Pattern | Expected UI Response |
|---|---|
| `SELECT *` from a table | CRUD: data-grid + "New record" button + sidebar filters |
| Column with FK to another table | Dropdown filter fetching related labels; drill-down link in cell |
| `GROUP BY date_trunc(...)` | Analytical: breakdown grid + KPI card (chart: future) |
| `COUNT(*), SUM(), AVG()` | Analytical: KPI cards in toolbar |
| Enum-like column (`status`, `type`) | CRUD: badge-filter or select in sidebar |
| User clicks a row | CRUD: navigate to detail/edit page |
| `INSERT INTO` or `UPDATE` query | CRUD: create/update the form page for that table |
| Multiple `DELETE` queries | CRUD: bulk checkbox column, "Delete selected" button |
| Column named `*_at` with timestamp type | Date range picker filter in sidebar |
| User repeats similar queries with different IDs | Parameterized search input in toolbar |

---

## PoC Scope

Single use case: **orders management**.

Schema:
```sql
CREATE TABLE customers (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    email       TEXT
);

CREATE TABLE orders (
    id          SERIAL PRIMARY KEY,
    customer_id INT REFERENCES customers(id),
    status      TEXT CHECK (status IN ('pending','processing','shipped','cancelled')),
    amount      NUMERIC(10,2),
    created_at  TIMESTAMPTZ DEFAULT now()
);
```

Target user journey:
1. `SELECT * FROM orders` → grid appears
2. Agent discovers FK → adds Customer filter dropdown
3. Agent sees `status` enum → adds status filter
4. User writes aggregate query → agent adds bar chart
5. User clicks a row → agent adds detail panel with Edit button
6. User edits → agent adds confirm dialog, switches grid to editable mode

---

## Development Phases

### Phase 1 — Go core (no LLM)
- [ ] pgx pool + query executor
- [ ] ActionLog (in-memory ring buffer, later Postgres)
- [ ] UIStateManager (in-memory component tree)
- [ ] WebSocket hub
- [ ] REST endpoints: `POST /query`, `GET /schema/:table`
- [ ] Hardcoded tool stubs returning mock results

### Phase 2 — React shell
- [ ] AG Grid bound to WS data stream
- [ ] SQL text input + submit
- [ ] WebSocket client
- [ ] UIEvent patch applier (Zustand store)
- [ ] Dynamic component renderer (switch on `type`)

### Phase 3 — Agent loop
- [ ] Anthropic Go SDK integration
- [ ] Tool registry wiring to real handlers
- [ ] Agent goroutine with cancellation
- [ ] System prompt (heuristics, output format)
- [ ] Debounce + concurrency control

### Phase 4 — UI components
- [ ] Chart component (Recharts, bound to grid data)
- [ ] Detail panel
- [ ] Insert/Edit modal
- [ ] KPI cards
- [ ] Bulk operation toolbar

---

## Environment Variables

```env
DATABASE_URL=postgres://user:pass@localhost:5432/adaptiveui
ANTHROPIC_API_KEY=sk-ant-...
PORT=8080
AGENT_MODEL=claude-sonnet-4-20250514
AGENT_MAX_TOKENS=4096
AGENT_DEBOUNCE_MS=300
```

---

## Key Constraints

- `run_query` tool is **read-only**: all SQL executed via this tool is wrapped in a read-only transaction. The agent cannot mutate data directly.
- Agent tool calls are **logged** for debugging (file or Postgres, configurable).
- UI state lives **only in memory** in the PoC — no persistence between sessions.
- The agent system prompt must instruct the model to **not duplicate components**: always call `get_current_ui()` before adding anything.
- Frontend must handle **out-of-order** UIEvents gracefully (WebSocket delivery is not guaranteed ordered under reconnect).

---

## Running Locally

```bash
# Start Postgres
docker-compose up -d postgres

# Seed schema + data
psql $DATABASE_URL -f scripts/seed.sql

# Run backend
go run ./cmd/server

# Run frontend
cd frontend && npm install && npm run dev
```
