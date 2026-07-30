package db

import "context"

// StatementInfo holds a row from pg_stat_statements for the current database.
type StatementInfo struct {
	QueryID     int64   `json:"query_id"`
	Query       string  `json:"query"`
	Calls       int64   `json:"calls"`
	TotalExecMs float64 `json:"total_exec_ms"`
	MeanExecMs  float64 `json:"mean_exec_ms"`
	Rows        int64   `json:"rows"`
}

// GetStatements returns query statistics from pg_stat_statements, filtered to
// the current database and excluding internal/system queries.
func (e *Executor) GetStatements(ctx context.Context) ([]StatementInfo, error) {
	const q = `
		SELECT
			pss.queryid,
			pss.query,
			pss.calls,
			pss.total_exec_time,
			pss.mean_exec_time,
			pss.rows
		FROM pg_stat_statements pss
		JOIN pg_database pd ON pd.oid = pss.dbid
		WHERE pd.datname = current_database()
		  AND pss.query NOT ILIKE '%pg_stat_statements%'
		  AND pss.query NOT ILIKE 'BEGIN%'
		  AND pss.query NOT ILIKE 'COMMIT%'
		  AND pss.query NOT ILIKE 'ROLLBACK%'
		  AND pss.query NOT ILIKE 'SET %'
		  AND pss.query NOT ILIKE 'SHOW %'
		  AND pss.query NOT ILIKE '%information_schema%'
		  AND pss.query NOT ILIKE '%pg_catalog%'
		  AND length(pss.query) >= 10
		ORDER BY pss.calls DESC
		LIMIT 200`

	rows, err := e.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stmts []StatementInfo
	for rows.Next() {
		var s StatementInfo
		if err := rows.Scan(&s.QueryID, &s.Query, &s.Calls, &s.TotalExecMs, &s.MeanExecMs, &s.Rows); err != nil {
			return nil, err
		}
		stmts = append(stmts, s)
	}
	return stmts, rows.Err()
}
