package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QueryResult struct {
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Error   string           `json:"error,omitempty"`
}

type Executor struct {
	pool *pgxpool.Pool
}

func NewExecutor(pool *pgxpool.Pool) *Executor {
	return &Executor{pool: pool}
}

func (e *Executor) Query(ctx context.Context, sql string) QueryResult {
	rows, err := e.pool.Query(ctx, sql)
	if err != nil {
		return QueryResult{Error: err.Error()}
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	cols := make([]string, len(fields))
	for i, f := range fields {
		cols[i] = string(f.Name)
	}

	var result []map[string]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return QueryResult{Error: err.Error()}
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = formatValue(vals[i])
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return QueryResult{Error: err.Error()}
	}

	return QueryResult{Columns: cols, Rows: result}
}

// QueryReadOnly wraps the query in a read-only transaction (for agent use).
func (e *Executor) QueryReadOnly(ctx context.Context, sql string, maxRows int) QueryResult {
	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return QueryResult{Error: err.Error()}
	}
	defer tx.Rollback(ctx)

	limitedSQL := fmt.Sprintf("SELECT * FROM (%s) _q LIMIT %d", sql, maxRows)
	rows, err := tx.Query(ctx, limitedSQL)
	if err != nil {
		return QueryResult{Error: err.Error()}
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	cols := make([]string, len(fields))
	for i, f := range fields {
		cols[i] = string(f.Name)
	}

	var result []map[string]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return QueryResult{Error: err.Error()}
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = formatValue(vals[i])
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return QueryResult{Error: err.Error()}
	}

	return QueryResult{Columns: cols, Rows: result}
}

func formatValue(v any) any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []byte:
		return string(t)
	case string:
		return t
	case bool, int16, int32, int64, float32, float64:
		return t
	default:
		// Use JSON round-trip: handles pgtype.Numeric → number, time.Time → RFC3339, etc.
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		var out any
		if err := json.Unmarshal(b, &out); err != nil {
			return fmt.Sprintf("%v", t)
		}
		return out
	}
}
