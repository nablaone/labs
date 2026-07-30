package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ColumnInfo struct {
	Name       string `json:"name"`
	DataType   string `json:"data_type"`
	Nullable   bool   `json:"nullable"`
	IsPrimary  bool   `json:"is_primary"`
	ForeignKey string `json:"foreign_key,omitempty"` // "table.column" if FK
}

type SchemaInfo struct {
	Table   string       `json:"table"`
	Columns []ColumnInfo `json:"columns"`
	Error   string       `json:"error,omitempty"`
}

type Inspector struct {
	pool *pgxpool.Pool
}

func NewInspector(pool *pgxpool.Pool) *Inspector {
	return &Inspector{pool: pool}
}

func (s *Inspector) Inspect(ctx context.Context, table string) SchemaInfo {
	const q = `
		SELECT
			c.column_name,
			c.data_type,
			(c.is_nullable = 'YES') AS nullable,
			COALESCE(tc.constraint_type = 'PRIMARY KEY', false) AS is_primary,
			COALESCE(ccu.table_name || '.' || ccu.column_name, '') AS foreign_key
		FROM information_schema.columns c
		LEFT JOIN information_schema.key_column_usage kcu
			ON kcu.table_name = c.table_name AND kcu.column_name = c.column_name
			AND kcu.table_schema = c.table_schema
		LEFT JOIN information_schema.table_constraints tc
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		LEFT JOIN information_schema.referential_constraints rc
			ON rc.constraint_name = kcu.constraint_name
		LEFT JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = rc.unique_constraint_name
		WHERE c.table_schema = 'public'
			AND c.table_name = $1
		ORDER BY c.ordinal_position`

	rows, err := s.pool.Query(ctx, q, table)
	if err != nil {
		return SchemaInfo{Table: table, Error: err.Error()}
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		if err := rows.Scan(&col.Name, &col.DataType, &col.Nullable, &col.IsPrimary, &col.ForeignKey); err != nil {
			return SchemaInfo{Table: table, Error: err.Error()}
		}
		cols = append(cols, col)
	}
	return SchemaInfo{Table: table, Columns: cols}
}

func (s *Inspector) Tables(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' ORDER BY table_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, nil
}

// AllTablesSchema returns schema info for every table in the public schema.
func (s *Inspector) AllTablesSchema(ctx context.Context) (map[string]SchemaInfo, error) {
	tables, err := s.Tables(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]SchemaInfo, len(tables))
	for _, t := range tables {
		result[t] = s.Inspect(ctx, t)
	}
	return result, nil
}
