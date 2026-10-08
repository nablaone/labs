package store

import (
	"database/sql"
	"encoding/json"
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
	// v3: agents (sub-users owned by a user).
	for _, c := range [][2]string{
		{"owner_id", "TEXT NOT NULL DEFAULT ''"},
		{"revoked", "INTEGER NOT NULL DEFAULT 0"},
		{"last_used_at", "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := addColumn(s.db, "users", c[0], c[1]); err != nil {
			return err
		}
	}
	if err := s.backfillFolders(); err != nil {
		return err
	}
	// v4: positions are objects (kind "position") with metadata in items.meta.
	if err := addColumn(s.db, "items", "meta", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumn(s.db, "users", "position_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return s.positionsToObjects()
}

// positionsToObjects gives every user a position id and turns rows of the old
// positions table into position objects in the users' folders.
func (s *Store) positionsToObjects() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id FROM users WHERE position_id = ''`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if _, err := tx.Exec(`UPDATE users SET position_id = ? WHERE id = ?`, NewUUID(), id); err != nil {
			return err
		}
	}
	rows, err = tx.Query(`SELECT u.position_id, u.team_id, u.folder_id, u.id, u.callsign, p.lat, p.lon, p.acc, p.hdg, p.spd, p.ts
		FROM positions p JOIN users u ON u.id = p.user_id
		WHERE NOT EXISTS (SELECT 1 FROM items WHERE items.id = u.position_id)`)
	if err != nil {
		return err
	}
	type old struct {
		posID, team, folder, user, callsign string
		lat, lon, acc                       float64
		hdg, spd                            *float64
		ts                                  int64
	}
	var olds []old
	for rows.Next() {
		var o old
		if err := rows.Scan(&o.posID, &o.team, &o.folder, &o.user, &o.callsign, &o.lat, &o.lon, &o.acc, &o.hdg, &o.spd, &o.ts); err != nil {
			rows.Close()
			return err
		}
		olds = append(olds, o)
	}
	rows.Close()
	for _, o := range olds {
		coords, _ := json.Marshal([][2]float64{{o.lat, o.lon}})
		meta, _ := json.Marshal(PosMeta{Source: "gps", Acc: o.acc, Hdg: o.hdg, Spd: o.spd, Fix: o.ts})
		if _, err := tx.Exec(`INSERT INTO items (id, team_id, kind, name, coords, folder, created_by, updated_by, updated_at, meta)
			VALUES (?, ?, 'position', ?, ?, ?, ?, ?, ?, ?)`, o.posID, o.team, o.callsign, string(coords), o.folder, o.user, o.user, o.ts, string(meta)); err != nil {
			return fmt.Errorf("migrate position of %s: %w", o.callsign, err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM positions`); err != nil {
		return err
	}
	return tx.Commit()
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
