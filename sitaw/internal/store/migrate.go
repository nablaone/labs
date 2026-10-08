package store

import (
	"database/sql"
	"fmt"
	"time"
)

// migrate brings databases created by older versions up to the current
// schema. Each step is idempotent, so it runs on every Open.
func (s *Store) migrate() error {
	// v2: folders.
	if err := addColumn(s.db, "items", "folder", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumn(s.db, "users", "folder_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS items_folder ON items(folder)`); err != nil {
		return err
	}
	return s.backfillFolders()
}

// backfillFolders gives every user a personal folder and files every map
// object that has no folder under its creator's folder.
func (s *Store) backfillFolders() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id, team_id, callsign, created_at FROM users WHERE folder_id = ''`)
	if err != nil {
		return err
	}
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.TeamID, &u.Callsign, &u.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		users = append(users, u)
	}
	rows.Close()
	for _, u := range users {
		u.FolderID = NewUUID()
		if err := createPersonalFolder(tx, u); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE users SET folder_id = ? WHERE id = ?`, u.FolderID, u.ID); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`UPDATE items SET folder = (SELECT folder_id FROM users WHERE users.id = items.created_by)
		WHERE kind != 'folder' AND folder = '' AND created_by IN (SELECT id FROM users)`)
	if err != nil {
		return err
	}
	// Objects whose creator is gone land in a shared "Unsorted" folder per team.
	teams, err := tx.Query(`SELECT DISTINCT team_id FROM items WHERE kind != 'folder' AND folder = ''`)
	if err != nil {
		return err
	}
	var orphanTeams []string
	for teams.Next() {
		var t string
		if err := teams.Scan(&t); err != nil {
			teams.Close()
			return err
		}
		orphanTeams = append(orphanTeams, t)
	}
	teams.Close()
	for _, t := range orphanTeams {
		id := NewUUID()
		if _, err := tx.Exec(`INSERT INTO items (id, team_id, kind, name, created_by, updated_by, updated_at)
			VALUES (?, ?, 'folder', 'Unsorted', '', '', ?)`, id, t, time.Now().UnixMilli()); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE items SET folder = ? WHERE team_id = ? AND kind != 'folder' AND folder = ''`, id, t); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// createPersonalFolder inserts the folder item for u (u.FolderID, named after the callsign).
func createPersonalFolder(tx *sql.Tx, u User) error {
	_, err := tx.Exec(`INSERT INTO items (id, team_id, kind, name, created_by, updated_by, updated_at)
		VALUES (?, ?, 'folder', ?, ?, ?, ?)`, u.FolderID, u.TeamID, u.Callsign, u.ID, u.ID, time.Now().UnixMilli())
	return err
}

func addColumn(db *sql.DB, table, col, def string) error {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if name == col {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	if _, err := db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, col, def)); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, col, err)
	}
	return nil
}
