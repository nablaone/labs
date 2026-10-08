package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Agents are sub-users: a users row with owner_id set. An agent reads all
// of its team's data but may only write map objects in its own folder (see
// PutItem). Its access token is the row's credential, prefixed so a leaked
// one is recognisable. Callsigns are <OWNER>-<NATO letter>.

// AgentTokenPrefix starts every agent access token.
const AgentTokenPrefix = "sitaw_at_"

var nato = []string{"ALPHA", "BRAVO", "CHARLIE", "DELTA", "ECHO", "FOXTROT", "GOLF", "HOTEL", "INDIA",
	"JULIETT", "KILO", "LIMA", "MIKE", "NOVEMBER", "OSCAR", "PAPA", "QUEBEC", "ROMEO", "SIERRA", "TANGO",
	"UNIFORM", "VICTOR", "WHISKEY", "XRAY", "YANKEE", "ZULU"}

// Agent is what an owner sees about one of their agents.
type Agent struct {
	ID         string `json:"id"`
	Callsign   string `json:"callsign"`
	FolderID   string `json:"folderId"`
	CreatedAt  int64  `json:"createdAt"`
	LastUsedAt int64  `json:"lastUsedAt,omitempty"`
	Revoked    bool   `json:"revoked,omitempty"`
}

// revokeSQL disables tokens: the hash is replaced by a value no token hashes to.
const revokeSQL = `UPDATE users SET revoked = 1, token_hash = 'revoked:' || id`

// CreateAgent creates the next agent of owner (callsign <OWNER>-ALPHA, -BRAVO,
// ... then -ALPHA-2 ...), with its personal folder, and returns it with its token.
func (s *Store) CreateAgent(owner User) (User, string, error) {
	if owner.IsAgent() {
		return User{}, "", ErrForbidden
	}
	tx, err := s.db.Begin()
	if err != nil {
		return User{}, "", err
	}
	defer tx.Rollback()

	taken := map[string]bool{}
	rows, err := tx.Query(`SELECT lower(callsign) FROM users WHERE team_id = ?`, owner.TeamID)
	if err != nil {
		return User{}, "", err
	}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			rows.Close()
			return User{}, "", err
		}
		taken[c] = true
	}
	rows.Close()
	callsign := ""
	for round := 1; callsign == ""; round++ {
		for _, n := range nato {
			c := strings.ToUpper(owner.Callsign) + "-" + n
			if round > 1 {
				c += fmt.Sprintf("-%d", round)
			}
			if !taken[strings.ToLower(c)] {
				callsign = c
				break
			}
		}
	}

	token := AgentTokenPrefix + randHex(24)
	a := User{ID: NewUUID(), TeamID: owner.TeamID, Callsign: callsign, FolderID: NewUUID(), OwnerID: owner.ID, CreatedAt: time.Now().UnixMilli()}
	if _, err := tx.Exec(`INSERT INTO users (id, team_id, callsign, token_hash, created_at, folder_id, owner_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.TeamID, a.Callsign, hashToken(token), a.CreatedAt, a.FolderID, a.OwnerID); err != nil {
		return User{}, "", err
	}
	if err := createPersonalFolder(tx, a); err != nil {
		return User{}, "", err
	}
	return a, token, tx.Commit()
}

// Agents lists the agents owned by ownerID, oldest first.
func (s *Store) Agents(ownerID string) ([]Agent, error) {
	rows, err := s.db.Query(`SELECT id, callsign, folder_id, created_at, last_used_at, revoked FROM users
		WHERE owner_id = ? ORDER BY created_at`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Agent{}
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.Callsign, &a.FolderID, &a.CreatedAt, &a.LastUsedAt, &a.Revoked); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// RevokeAgent disables an agent's token. The agent stays in the roster (as
// revoked) so its folder and objects keep their author.
func (s *Store) RevokeAgent(ownerID, agentID string) (PublicUser, error) {
	var u PublicUser
	err := s.db.QueryRow(revokeSQL+` WHERE id = ? AND owner_id = ? RETURNING id, callsign, folder_id, owner_id, revoked`,
		agentID, ownerID).Scan(&u.ID, &u.Callsign, &u.FolderID, &u.OwnerID, &u.Revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return PublicUser{}, ErrNotFound
	}
	return u, err
}

// TouchUser records that a user (in practice: an agent) used its token.
// Writes at most once a minute per user.
func (s *Store) TouchUser(id string) error {
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(`UPDATE users SET last_used_at = ? WHERE id = ? AND last_used_at < ?`, now, id, now-60_000)
	return err
}
