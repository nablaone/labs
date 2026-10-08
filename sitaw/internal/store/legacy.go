package store

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// legacyData is the pre-SQLite JSON state file (single team, no team ids).
type legacyData struct {
	Users map[string]struct {
		ID        string `json:"id"`
		Callsign  string `json:"callsign"`
		TokenHash string `json:"tokenHash"`
		CreatedAt int64  `json:"createdAt"`
	} `json:"users"`
	Invites   map[string]Invite   `json:"invites"`
	Items     map[string]Item     `json:"items"`
	Positions map[string]Position `json:"positions"`
}

// ImportLegacyJSON moves a pre-SQLite state file into a new team called
// name, keeping user ids and token hashes so existing sessions keep working.
// On success the file is renamed to <path>.imported. Returns the new team.
func (s *Store) ImportLegacyJSON(path, name string) (Team, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Team{}, err
	}
	var d legacyData
	if err := json.Unmarshal(b, &d); err != nil {
		return Team{}, fmt.Errorf("parse %s: %w", path, err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Team{}, err
	}
	defer tx.Rollback()
	t := Team{ID: NewUUID(), Name: name, CreatedAt: time.Now().UnixMilli()}
	if _, err := tx.Exec(`INSERT INTO teams (id, name, created_at) VALUES (?, ?, ?)`, t.ID, t.Name, t.CreatedAt); err != nil {
		return Team{}, err
	}
	for _, inv := range d.Invites {
		if _, err := tx.Exec(`INSERT INTO invites (token, team_id, created_at, expires_at, revoked, uses) VALUES (?, ?, ?, ?, ?, ?)`,
			inv.Token, t.ID, inv.CreatedAt, inv.ExpiresAt, inv.Revoked, inv.Uses); err != nil {
			return Team{}, fmt.Errorf("invite: %w", err)
		}
	}
	for _, u := range d.Users {
		if _, err := tx.Exec(`INSERT INTO users (id, team_id, callsign, token_hash, created_at) VALUES (?, ?, ?, ?, ?)`,
			u.ID, t.ID, u.Callsign, u.TokenHash, u.CreatedAt); err != nil {
			return Team{}, fmt.Errorf("user %s: %w", u.Callsign, err)
		}
	}
	for _, it := range d.Items {
		coords, _ := json.Marshal(it.Coords)
		if _, err := tx.Exec(`INSERT INTO items (id, team_id, kind, name, color, remarks, coords, created_by, updated_by, updated_at, deleted)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			it.ID, t.ID, it.Kind, it.Name, it.Color, it.Remarks, string(coords), it.CreatedBy, it.UpdatedBy, it.UpdatedAt, it.Deleted); err != nil {
			return Team{}, fmt.Errorf("item %s: %w", it.ID, err)
		}
	}
	for _, p := range d.Positions {
		if _, ok := d.Users[p.UserID]; !ok {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO positions (user_id, team_id, callsign, lat, lon, acc, hdg, spd, ts) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.UserID, t.ID, p.Callsign, p.Lat, p.Lon, p.Accuracy, p.Heading, p.Speed, p.TS); err != nil {
			return Team{}, fmt.Errorf("position: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Team{}, err
	}
	if err := s.backfillFolders(); err != nil {
		return Team{}, err
	}
	return t, os.Rename(path, path+".imported")
}
