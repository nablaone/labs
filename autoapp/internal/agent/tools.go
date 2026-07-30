package agent

import (
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
)

// toolDefs returns the full list of tool definitions sent to the LLM.
func toolDefs() []anthropic.ToolUnionParam {
	schema := func(props map[string]any, required []string) anthropic.ToolInputSchemaParam {
		return anthropic.ToolInputSchemaParam{
			Properties: props,
			Required:   required,
		}
	}

	tools := []anthropic.ToolParam{
		{
			Name:        "inspect_schema",
			Description: anthropic.String("Get column metadata for a database table. Returns column names, types, primary-key and foreign-key flags."),
			InputSchema: schema(map[string]any{
				"table": map[string]any{"type": "string", "description": "Table name to inspect"},
			}, []string{"table"}),
		},
		{
			Name:        "run_query",
			Description: anthropic.String("Execute a read-only SQL query (max 50 rows). Use this to explore data before building UI components."),
			InputSchema: schema(map[string]any{
				"sql": map[string]any{"type": "string", "description": "SQL SELECT statement"},
			}, []string{"sql"}),
		},
		{
			Name:        "get_action_history",
			Description: anthropic.String("Return the last N user actions (queries, clicks, edits). Use this to understand user intent."),
			InputSchema: schema(map[string]any{
				"n": map[string]any{"type": "integer", "description": "Number of recent actions to fetch (default 20)"},
			}, nil),
		},
		{
			Name:        "get_current_ui",
			Description: anthropic.String("Return the current UI component tree. Always call this before add_component to avoid duplicates."),
			InputSchema: schema(map[string]any{}, nil),
		},
		{
			Name:        "add_component",
			Description: anthropic.String("Add a new UI component. Use type 'page' to create a new page in the nav. For all other types, set page_id to place the component on a specific page. Triggers an immediate WebSocket broadcast to the frontend."),
			InputSchema: schema(map[string]any{
				"id":      map[string]any{"type": "string", "description": "Unique component ID (kebab-case)"},
				"type":    map[string]any{"type": "string", "description": "Component type: 'page' to create a nav page, or data-grid, kpi-card, chart, button, select, date-picker, detail-panel, modal, toolbar, tabs, badge-filter, pagination, notification, divider, text-input"},
				"label":   map[string]any{"type": "string", "description": "Human-readable label"},
				"page_id": map[string]any{"type": "string", "description": "Page this component belongs to (required for all non-page components). Must match an existing page id."},
				"props":   map[string]any{"type": "object", "description": "Component-specific props (see component library spec)"},
				"position": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"zone":  map[string]any{"type": "string", "enum": []string{"toolbar", "sidebar", "main", "footer"}},
						"order": map[string]any{"type": "integer"},
					},
				},
			}, []string{"id", "type"}),
		},
		{
			Name:        "update_component",
			Description: anthropic.String("Update props of an existing component. Triggers WebSocket broadcast."),
			InputSchema: schema(map[string]any{
				"id":    map[string]any{"type": "string"},
				"props": map[string]any{"type": "object"},
			}, []string{"id", "props"}),
		},
		{
			Name:        "remove_component",
			Description: anthropic.String("Remove a component from the UI. Triggers WebSocket broadcast."),
			InputSchema: schema(map[string]any{
				"id": map[string]any{"type": "string"},
			}, []string{"id"}),
		},
		{
			Name:        "navigate_to_page",
			Description: anthropic.String("Navigate the user's browser to a specific page. Call this after creating a new page to route the user there immediately."),
			InputSchema: schema(map[string]any{
				"page_id": map[string]any{"type": "string", "description": "ID of the page to navigate to"},
			}, []string{"page_id"}),
		},
		{
			Name:        "bind_action",
			Description: anthropic.String("Bind a trigger on a component to an action handler (SQL query, modal open, navigation, filter). Triggers WebSocket broadcast."),
			InputSchema: schema(map[string]any{
				"component_id": map[string]any{"type": "string"},
				"trigger":      map[string]any{"type": "string", "description": "e.g. click, change, row-click"},
				"handler": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type":    map[string]any{"type": "string", "enum": []string{"query", "modal", "nav", "filter"}},
						"payload": map[string]any{"type": "string"},
					},
					"required": []string{"type", "payload"},
				},
			}, []string{"component_id", "trigger", "handler"}),
		},
	}

	out := make([]anthropic.ToolUnionParam, len(tools))
	for i, t := range tools {
		tc := t
		out[i] = anthropic.ToolUnionParam{OfTool: &tc}
	}
	return out
}

// parseInput decodes a tool call's raw JSON input into a map.
func parseInput(block anthropic.ToolUseBlock) (map[string]any, error) {
	var m map[string]any
	raw := block.JSON.Input.Raw()
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}
	return m, nil
}

func strField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func intField(m map[string]any, key string, def int) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return def
}

func mapField(m map[string]any, key string) map[string]any {
	if v, ok := m[key]; ok {
		if mp, ok := v.(map[string]any); ok {
			return mp
		}
	}
	return nil
}
