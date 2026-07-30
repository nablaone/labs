package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"autoapp/internal/actionlog"
	"autoapp/internal/db"
	"autoapp/internal/schemacache"
	"autoapp/internal/uistate"
	"autoapp/internal/ws"
)

const systemPrompt = `You are an adaptive UI agent. You build and evolve a web UI autonomously based on two signal types:

1. DISCOVERY — new SQL query patterns found in pg_stat_statements from any database connection.
   Your job is to classify the query and build the right UI page for it.

2. UI INTERACTION — the user clicked a row, button, or submitted a form.
   Your job is to respond: navigate, fill in a detail page, open an edit form, etc.

## UI Structure

The UI is split into two panels:
- LEFT: navigation menu listing all pages (type "page" components)
- RIGHT: the active page, rendered in zones: toolbar (top), sidebar (left), main (center), footer (bottom)

There is always a built-in page "page-editor" (Query Editor) — never add components to it.

## Step 1 — Classify the query

Before building any UI, classify the SQL query into one of two intents:

### CRUD
Signals: plain SELECT on one table (no aggregates), INSERT, UPDATE, DELETE, or SELECT by PK.

CRUD pages: 

#### 1. List page  (e.g. id="page-orders", label="Orders")
- Filters at the top of the main zone (low order numbers) — one control per filterable column:
  text-input for name/text columns, select for enum/status/badge columns, date-picker for _at/_date columns.
  FK columns get a select whose options are fetched from the referenced table via run_query.
- data-grid below the filters with:
  - All visible columns (hide_in_ui=false), business_name as header, display_hint as type
  - FK columns: rewrite SQL with LEFT JOIN to show the display column, never the raw integer ID
  - An "Actions" column as the last column: {"field":"actions","header":"Actions","type":"actions","sortable":false,"width":160,"actions":[{"label":"Details","navigate":"page-X-detail"},{"label":"Edit","navigate":"page-X-edit"}]}
    Replace "page-X-detail" and "page-X-edit" with the actual page IDs for that entity.
  - Embed rows from run_query in props.rows
- "New [Entity]" button in toolbar

#### 2. Detail page  (e.g. id="page-order-detail", label="Order Detail")
- Read-only display of ALL non-hidden columns. Use a detail-panel component or individual labeled text fields.
- FK columns: run a JOIN query to show the referenced entity name, never a raw integer.
- Toolbar with exactly three buttons:
  - "Edit" → navigate_to_page to the edit page
  - "Delete" → button with confirmDialog showing a confirmation message, variant "danger"
  - "Back to List" → navigate_to_page to the list page
- When the agent navigates here from a row click, update the detail components with the selected row's data.

#### 3. Edit page  (e.g. id="page-order-edit", label="Edit Order")
- One input per editable column (all columns except PK):
  - text columns → text-input
  - numeric columns → text-input
  - date/_at columns → date-picker
  - enum/status badge columns → select with static options list
  - FK columns → call run_query("SELECT id, display_col FROM ref_table LIMIT 21"):
    if result has 21 rows or fewer: select component with options fetched from the query
    if result has more than 20 rows: text-input with placeholder naming the referenced entity
- Toolbar with exactly two buttons: "Cancel" (navigate back to detail page), "Save"

Add corresponding page if you think the user may want to use it. Do not add all the functionality up front.
If the user writes a plain select with *, just add a list. If clicks on a row, then add the detail page. If clicks "Edit", then add the edit page. Follow the user's lead. 
If the use deletes a single record add a "Delete" button on the detail page.
If the user deletes multiple records at once, add a delete button on the list page and checkboxes to select multiple rows.
Try to be one step ahead of the user.

NEVER add KPI cards, charts, or breakdown grids for CRUD queries.

### Analytical
Signals: GROUP BY, COUNT/SUM/AVG/MIN/MAX, date_trunc, multi-table JOIN with aggregation, HAVING, CTEs computing metrics.
Build a read-only analytics surface:
- kpi-card in toolbar for each scalar metric (COUNT, SUM, AVG)
- chart in main — pick type using the rules below
- data-grid in main below the chart for the full breakdown rows
NEVER add New/Edit/Delete buttons or form pages for analytical queries.

### Chart type selection

| Data pattern | Chart type |
|---|---|
| GROUP BY timestamp / date_trunc | line — shows trend over time |
| GROUP BY timestamp, cumulative total | area — shows volume over time |
| GROUP BY enum/status/category, ≤ 8 groups, values sum to a whole | pie |
| GROUP BY category, > 8 groups or comparison focus | bar |
| Two numeric columns, no GROUP BY | scatter |

Decision shortcuts:
- "Distribution across statuses/types?" → pie (≤ 8 slices) or bar (> 8)
- "Change over time?" → line
- "Which category has the most?" → bar
- "Volume over time?" → area

Chart props must include rows[] data inline, xAxis.field, yAxis[].field, height (default 280):
{"chartType":"pie","rows":[{"status":"shipped","count":4}],"xAxis":{"field":"status"},"yAxis":[{"field":"count"}],"height":280}

A page must never mix CRUD controls with analytical components.
If the user writes both query types for the same table, maintain two separate pages.

## Schema Annotations

When inspect_schema returns data, each column includes business metadata:
- **business_name** — use this as column header and form label; never use the raw column name
- **hide_in_ui** — if true, omit this column from grids and forms entirely
- **display_hint** — use as the column type in data-grid:
  - "pk" → skip (always hidden)
  - "fk" → skip the raw integer ID; JOIN the referenced table instead (see FK rule below)
  - "badge" → type: "badge"
  - "currency" → type: "currency"
  - "datetime" → type: "datetime"
  - "date" → type: "date"
  - "email" → type: "link"
  - "text" → type: "text"
  - "number" → type: "number"
- **fk_ref** — for FK columns: "referenced_table.display_column" (e.g. "customers.name")

**FK rule:** Never display raw FK integer IDs. For every FK column (display_hint="fk"), rewrite the SQL to LEFT JOIN the referenced table and select its display column. Example: if orders has customer_id with fk_ref "customers.name", add LEFT JOIN customers c ON c.id = o.customer_id and select c.name AS customer_name, then use "customer_name" in the grid with business_name "Customer".

## Step 2 — Pages

Create a new page for each distinct entity or view. Never reuse a page for a different intent.
To create a page: add_component({ id: "page-orders", type: "page", label: "Orders" })
All non-page components must have page_id set to an existing page.

## Step 3 — Navigation

Never use modal components. Instead, create a dedicated page and navigate to it:
1. add_component({ id: "page-new-order", type: "page", label: "New Order" })
2. Add form fields with page_id: "page-new-order"
3. navigate_to_page({ page_id: "page-new-order" })

Always call navigate_to_page after creating a page that should be shown immediately.

## Component types

data-grid, text-input, button, select, date-picker, kpi-card, chart,
detail-panel, toolbar, tabs, badge-filter, pagination, notification, divider.
NEVER use "modal".

## Rules

1. ALWAYS call get_current_ui() first — never duplicate an existing component.
2. Never add components to "page-editor".
3. Always set page_id on every non-page component.
4. Create the page before adding components to it.
5. run_query is read-only. Never mutate data.
6. DATA GRIDS NEED ROWS: Always call run_query to fetch actual data, then embed the result rows in the data-grid props as the "rows" array. The grid has no live data source — it only shows what is in props.rows. Pass rows from run_query directly into add_component props.
8. KPI CARDS NEED VALUES: Run the aggregate query and embed the numeric result as the "value" prop.
9. CHARTS NEED ROWS: Run the query and embed rows inline in chart props.`

func (a *Agent) buildUserMessage(trigger actionlog.ActionEvent) string {
	b, _ := json.Marshal(trigger.Payload)
	switch trigger.Type {
	case actionlog.EventDiscovery:
		return fmt.Sprintf(
			"New SQL query patterns were detected in pg_stat_statements:\n%s\n\n"+
				"For each query: classify it (CRUD or Analytical), check the current UI, "+
				"and build the appropriate page if one does not already exist.",
			string(b))
	default:
		tb, _ := json.Marshal(trigger)
		return fmt.Sprintf(
			"The user performed a UI interaction:\n%s\n\n"+
				"Respond appropriately — navigate to a page, populate a detail view, "+
				"open an edit form, or update components as needed.",
			string(tb))
	}
}

// Agent runs an LLM loop that observes user actions and emits UI mutations.
type Agent struct {
	client      *anthropic.Client
	exec        *db.Executor
	inspector   *db.Inspector
	schemaCache *schemacache.Cache
	alog        *actionlog.Log
	ui          *uistate.Manager
	hub         *ws.Hub
}

func New(apiKey string, exec *db.Executor, inspector *db.Inspector, sc *schemacache.Cache, alog *actionlog.Log, ui *uistate.Manager, hub *ws.Hub) *Agent {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Agent{
		client:      &client,
		exec:        exec,
		inspector:   inspector,
		schemaCache: sc,
		alog:        alog,
		ui:          ui,
		hub:         hub,
	}
}

// Run executes one agent loop pass in response to a trigger event.
func (a *Agent) Run(ctx context.Context, trigger actionlog.ActionEvent) error {
	userMsg := a.buildUserMessage(trigger)

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg)),
	}

	tools := toolDefs()

	for {
		resp, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.ModelClaudeHaiku4_5,
			MaxTokens: 4096,
			System: []anthropic.TextBlockParam{
				{Text: systemPrompt},
			},
			Tools:    tools,
			Messages: messages,
		})
		if err != nil {
			return fmt.Errorf("llm call: %w", err)
		}

		messages = append(messages, resp.ToParam())

		if resp.StopReason != anthropic.StopReasonToolUse {
			break
		}

		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range resp.Content {
			toolUse, ok := block.AsAny().(anthropic.ToolUseBlock)
			if !ok {
				continue
			}
			result, isErr := a.executeTool(ctx, toolUse)
			if isErr {
				log.Printf("[agent] tool %s error: %s", toolUse.Name, result)
			} else {
				log.Printf("[agent] tool %s ok", toolUse.Name)
			}
			toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, result, isErr))
		}

		if len(toolResults) > 0 {
			messages = append(messages, anthropic.NewUserMessage(toolResults...))
		}
	}

	log.Printf("[agent] run complete\n%s", a.ui.DumpTree())
	return nil
}

func (a *Agent) executeTool(ctx context.Context, block anthropic.ToolUseBlock) (string, bool) {
	input, err := parseInput(block)
	if err != nil {
		return err.Error(), true
	}

	switch block.Name {
	case "get_action_history":
		n := intField(input, "n", 20)
		events := a.alog.Last(n)
		b, _ := json.Marshal(events)
		return string(b), false

	case "get_current_ui":
		tree := a.ui.Tree()
		b, _ := json.Marshal(tree)
		return string(b), false

	case "inspect_schema":
		table := strField(input, "table")
		if table == "" {
			return "table parameter required", true
		}
		raw := a.inspector.Inspect(ctx, table)

		// Merge raw schema with cached business metadata
		cached := a.schemaCache.GetTable(table)
		if cached == nil {
			b, _ := json.Marshal(raw)
			return string(b), false
		}

		type enrichedColumn struct {
			db.ColumnInfo
			BusinessName string `json:"business_name"`
			HideInUI     bool   `json:"hide_in_ui"`
			DisplayHint  string `json:"display_hint"`
			FKRef        string `json:"fk_ref,omitempty"`
		}
		type enrichedSchema struct {
			Table   string           `json:"table"`
			Label   string           `json:"label"`
			Columns []enrichedColumn `json:"columns"`
			Error   string           `json:"error,omitempty"`
		}

		metaByName := make(map[string]schemacache.ColumnMeta, len(cached.Columns))
		for _, cm := range cached.Columns {
			metaByName[cm.Name] = cm
		}

		cols := make([]enrichedColumn, 0, len(raw.Columns))
		for _, rc := range raw.Columns {
			ec := enrichedColumn{ColumnInfo: rc}
			if m, ok := metaByName[rc.Name]; ok {
				ec.BusinessName = m.BusinessName
				ec.HideInUI = m.HideInUI
				ec.DisplayHint = m.DisplayHint
				ec.FKRef = m.FKRef
			}
			cols = append(cols, ec)
		}
		result := enrichedSchema{
			Table:   raw.Table,
			Label:   cached.Label,
			Columns: cols,
			Error:   raw.Error,
		}
		b, _ := json.Marshal(result)
		return string(b), false

	case "run_query":
		sql := strField(input, "sql")
		if sql == "" {
			return "sql parameter required", true
		}
		result := a.exec.QueryReadOnly(ctx, sql, 50)
		if result.Error != "" {
			return result.Error, true
		}
		b, _ := json.Marshal(result)
		return string(b), false

	case "add_component":
		id := strField(input, "id")
		typ := strField(input, "type")
		if id == "" || typ == "" {
			return "id and type required", true
		}
		spec := uistate.ComponentSpec{
			ID:     id,
			Type:   typ,
			Label:  strField(input, "label"),
			PageID: strField(input, "page_id"),
			Props:  mapField(input, "props"),
		}
		if pos := mapField(input, "position"); pos != nil {
			spec.Position = &uistate.Position{
				Zone:  strField(pos, "zone"),
				Order: intField(pos, "order", 0),
			}
		}
		added, err := a.ui.Add(spec)
		if err != nil {
			// Component already exists — not a hard error, just report it
			return fmt.Sprintf("component already exists: %s", err), false
		}
		a.hub.Broadcast(uistate.UIEvent{Op: "add", Component: &added})
		return id, false

	case "update_component":
		id := strField(input, "id")
		props := mapField(input, "props")
		if id == "" {
			return "id required", true
		}
		if err := a.ui.Update(id, props); err != nil {
			return err.Error(), true
		}
		a.hub.Broadcast(uistate.UIEvent{Op: "update", ID: id, Props: props})
		return "ok", false

	case "remove_component":
		id := strField(input, "id")
		if id == "" {
			return "id required", true
		}
		if err := a.ui.Remove(id); err != nil {
			return err.Error(), true
		}
		a.hub.Broadcast(uistate.UIEvent{Op: "remove", ID: id})
		return "ok", false

	case "navigate_to_page":
		pageID := strField(input, "page_id")
		if pageID == "" {
			return "page_id required", true
		}
		a.hub.Broadcast(uistate.UIEvent{Op: "navigate", ID: pageID})
		return "ok", false

	case "bind_action":
		componentID := strField(input, "component_id")
		trigger := strField(input, "trigger")
		handler := mapField(input, "handler")
		if componentID == "" || trigger == "" || handler == nil {
			return "component_id, trigger, and handler required", true
		}
		a.hub.Broadcast(uistate.UIEvent{
			Op:      "bind",
			ID:      componentID,
			Trigger: trigger,
			Handler: handler,
		})
		return "ok", false

	default:
		return fmt.Sprintf("unknown tool: %s", block.Name), true
	}
}
