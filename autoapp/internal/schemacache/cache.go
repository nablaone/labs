package schemacache

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"autoapp/internal/db"
)

// ColumnMeta holds enriched metadata for a single column.
type ColumnMeta struct {
	Name         string `json:"name"`
	DataType     string `json:"data_type"`
	Nullable     bool   `json:"nullable"`
	IsPK         bool   `json:"is_pk"`
	IsFK         bool   `json:"is_fk"`
	FKRef        string `json:"fk_ref,omitempty"` // "table.column" pointing at display column
	BusinessName string `json:"business_name"`
	HideInUI     bool   `json:"hide_in_ui"`
	DisplayHint  string `json:"display_hint"` // pk|fk|badge|currency|datetime|date|email|text|number
}

// TableMeta holds enriched metadata for a single table.
type TableMeta struct {
	Label   string       `json:"label"`
	Columns []ColumnMeta `json:"columns"`
}

// Cache is the persisted schema cache with business-logic annotations.
type Cache struct {
	SHA1       string               `json:"sha1"`
	AnalyzedAt time.Time            `json:"analyzed_at"`
	Tables     map[string]TableMeta `json:"tables"`
}

// GetTable returns enriched metadata for a table, or nil if not cached.
func (c *Cache) GetTable(table string) *TableMeta {
	if c == nil {
		return nil
	}
	if t, ok := c.Tables[table]; ok {
		cp := t
		return &cp
	}
	return nil
}

// LoadOrRefresh loads the cache from filePath, re-analyzing with LLM if schema changed.
func LoadOrRefresh(ctx context.Context, inspector *db.Inspector, apiKey, filePath string) (*Cache, error) {
	raw, err := inspector.AllTablesSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspect all tables: %w", err)
	}

	currentHash := hashSchema(raw)

	if data, err := os.ReadFile(filePath); err == nil {
		var cached Cache
		if json.Unmarshal(data, &cached) == nil && cached.SHA1 == currentHash {
			log.Printf("[schema] cache hit (sha1=%s)", currentHash[:8])
			return &cached, nil
		}
		log.Printf("[schema] schema changed, re-analyzing")
	}

	var cache *Cache
	if apiKey != "" {
		cache, err = analyzWithLLM(ctx, apiKey, currentHash, raw)
		if err != nil {
			log.Printf("[schema] LLM analysis failed: %v — falling back to heuristics", err)
			cache = buildHeuristicCache(currentHash, raw)
		} else {
			log.Printf("[schema] LLM analysis complete")
		}
	} else {
		log.Printf("[schema] no API key, using heuristic cache")
		cache = buildHeuristicCache(currentHash, raw)
	}

	data, _ := json.MarshalIndent(cache, "", "  ")
	if writeErr := os.WriteFile(filePath, data, 0644); writeErr != nil {
		log.Printf("[schema] could not save cache: %v", writeErr)
	}
	return cache, nil
}

func hashSchema(tables map[string]db.SchemaInfo) string {
	b, _ := json.Marshal(tables)
	return fmt.Sprintf("%x", sha1.Sum(b))
}

// analyzWithLLM calls Claude to annotate the schema with business metadata.
func analyzWithLLM(ctx context.Context, apiKey, hash string, tables map[string]db.SchemaInfo) (*Cache, error) {
	schemaJSON, _ := json.MarshalIndent(tables, "", "  ")

	prompt := fmt.Sprintf(`Analyze this PostgreSQL schema and return business metadata for the UI.

Schema:
%s

Return ONLY valid JSON (no prose, no markdown fences) in exactly this shape:
{
  "tables": {
    "<table_name>": {
      "label": "<human-readable table label>",
      "columns": [
        {
          "name": "<exact column name>",
          "business_name": "<human-readable label without underscores or _id suffix>",
          "hide_in_ui": <true|false>,
          "display_hint": "<pk|fk|badge|currency|datetime|date|email|text|number>",
          "fk_ref": "<referenced_table.display_column or empty string>"
        }
      ]
    }
  }
}

Rules:
- Primary key columns: hide_in_ui=true, display_hint="pk"
- Foreign key integer ID columns (e.g. customer_id): hide_in_ui=true, display_hint="fk", fk_ref points to the most useful display column in the referenced table (e.g. "customers.name")
- amount/price/cost/total columns: display_hint="currency"
- _at / _time / timestamp columns: display_hint="datetime"
- _date columns: display_hint="date"
- status/type/state/kind/category columns (likely enum): display_hint="badge"
- email columns: display_hint="email"
- Other text: display_hint="text", Other numeric: display_hint="number"
- business_name must be clean English words, no underscores, no "_id" suffix`, string(schemaJSON))

	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: 8096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("empty LLM response")
	}

	raw := strings.TrimSpace(resp.Content[0].Text)
	// Strip markdown code fences if present
	if strings.HasPrefix(raw, "```") {
		lines := strings.SplitN(raw, "\n", 2)
		if len(lines) == 2 {
			raw = lines[1]
		}
		raw = strings.TrimSuffix(strings.TrimSpace(raw), "```")
	}

	var parsed struct {
		Tables map[string]struct {
			Label   string `json:"label"`
			Columns []struct {
				Name         string `json:"name"`
				BusinessName string `json:"business_name"`
				HideInUI     bool   `json:"hide_in_ui"`
				DisplayHint  string `json:"display_hint"`
				FKRef        string `json:"fk_ref"`
			} `json:"columns"`
		} `json:"tables"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		snippet := raw
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return nil, fmt.Errorf("parse LLM response: %w (raw: %s)", err, snippet)
	}

	cache := &Cache{
		SHA1:       hash,
		AnalyzedAt: time.Now(),
		Tables:     make(map[string]TableMeta, len(tables)),
	}
	for tableName, rawInfo := range tables {
		pt, ok := parsed.Tables[tableName]
		label := titleCase(tableName)
		if ok && pt.Label != "" {
			label = pt.Label
		}

		metaByName := make(map[string]struct {
			BusinessName string
			HideInUI     bool
			DisplayHint  string
			FKRef        string
		}, len(pt.Columns))
		if ok {
			for _, cm := range pt.Columns {
				metaByName[cm.Name] = struct {
					BusinessName string
					HideInUI     bool
					DisplayHint  string
					FKRef        string
				}{cm.BusinessName, cm.HideInUI, cm.DisplayHint, cm.FKRef}
			}
		}

		cols := make([]ColumnMeta, 0, len(rawInfo.Columns))
		for _, rawCol := range rawInfo.Columns {
			m, hasMeta := metaByName[rawCol.Name]
			bname := toBusinessName(rawCol.Name)
			hint := guessHint(rawCol)
			hide := rawCol.IsPrimary || (rawCol.ForeignKey != "" && strings.HasSuffix(rawCol.Name, "_id"))
			fkRef := rawCol.ForeignKey

			if hasMeta {
				if m.BusinessName != "" {
					bname = m.BusinessName
				}
				hide = m.HideInUI
				if m.DisplayHint != "" {
					hint = m.DisplayHint
				}
				if m.FKRef != "" {
					fkRef = m.FKRef
				}
			}
			cols = append(cols, ColumnMeta{
				Name:         rawCol.Name,
				DataType:     rawCol.DataType,
				Nullable:     rawCol.Nullable,
				IsPK:         rawCol.IsPrimary,
				IsFK:         rawCol.ForeignKey != "",
				FKRef:        fkRef,
				BusinessName: bname,
				HideInUI:     hide,
				DisplayHint:  hint,
			})
		}
		cache.Tables[tableName] = TableMeta{Label: label, Columns: cols}
	}
	return cache, nil
}

// buildHeuristicCache creates a cache using only rule-based annotations (no LLM).
func buildHeuristicCache(hash string, tables map[string]db.SchemaInfo) *Cache {
	cache := &Cache{
		SHA1:       hash,
		AnalyzedAt: time.Now(),
		Tables:     make(map[string]TableMeta, len(tables)),
	}
	for tableName, info := range tables {
		cols := make([]ColumnMeta, 0, len(info.Columns))
		for _, col := range info.Columns {
			cols = append(cols, ColumnMeta{
				Name:         col.Name,
				DataType:     col.DataType,
				Nullable:     col.Nullable,
				IsPK:         col.IsPrimary,
				IsFK:         col.ForeignKey != "",
				FKRef:        col.ForeignKey,
				BusinessName: toBusinessName(col.Name),
				HideInUI:     col.IsPrimary || (col.ForeignKey != "" && strings.HasSuffix(col.Name, "_id")),
				DisplayHint:  guessHint(col),
			})
		}
		cache.Tables[tableName] = TableMeta{Label: titleCase(tableName), Columns: cols}
	}
	return cache
}

// guessHint applies heuristic rules to infer a display hint.
func guessHint(col db.ColumnInfo) string {
	if col.IsPrimary {
		return "pk"
	}
	if col.ForeignKey != "" {
		return "fk"
	}
	name := strings.ToLower(col.Name)
	dt := strings.ToLower(col.DataType)

	if strings.HasSuffix(name, "_at") || strings.HasSuffix(name, "_time") || strings.HasPrefix(dt, "timestamp") {
		return "datetime"
	}
	if strings.HasSuffix(name, "_date") || dt == "date" {
		return "date"
	}
	if name == "email" || strings.HasSuffix(name, "_email") {
		return "email"
	}
	if name == "status" || name == "state" || name == "type" || name == "kind" ||
		strings.HasSuffix(name, "_status") || strings.HasSuffix(name, "_state") || strings.HasSuffix(name, "_type") {
		return "badge"
	}
	if strings.Contains(name, "amount") || strings.Contains(name, "price") ||
		strings.Contains(name, "cost") || strings.Contains(name, "total") ||
		strings.Contains(name, "fee") || strings.Contains(name, "revenue") {
		return "currency"
	}
	if strings.Contains(dt, "int") || dt == "numeric" || dt == "decimal" ||
		dt == "float4" || dt == "float8" || dt == "real" || dt == "double precision" {
		return "number"
	}
	return "text"
}

// toBusinessName converts a snake_case column name to a human-readable label.
func toBusinessName(name string) string {
	name = strings.TrimSuffix(name, "_id")
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// titleCase converts a snake_case name to Title Case.
func titleCase(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
