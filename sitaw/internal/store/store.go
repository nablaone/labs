// Package store keeps all sitaw state in SQLite. Every row belongs to a team
// and every query is scoped by team, so one server can host many teams
// without them seeing each other's data.
package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Item kinds.
const (
	KindWaypoint = "waypoint"
	KindLine     = "line"
	KindArea     = "area"
	// KindFolder groups other items (flat: folders don't nest). Every user has
	// a personal folder; anyone in the team can create more.
	KindFolder = "folder"
)

type Team struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"createdAt"`
}

// Item is a shared object: a map object (waypoint, line, area) or a folder.
// Deletion is a tombstone (Deleted=true) so offline clients converge on reconnect.
type Item struct {
	ID        string       `json:"id"`
	Kind      string       `json:"kind"`
	Name      string       `json:"name"`
	Color     string       `json:"color,omitempty"`
	Remarks   string       `json:"remarks,omitempty"`
	Coords    [][2]float64 `json:"coords"`           // [lat, lon]; empty for folders
	Folder    string       `json:"folder,omitempty"` // folder id; empty for folders
	CreatedBy string       `json:"createdBy"`
	UpdatedBy string       `json:"updatedBy"`
	UpdatedAt int64        `json:"updatedAt"` // unix ms, set by the editing client
	Deleted   bool         `json:"deleted,omitempty"`
}

// Position is the latest known location of a user.
type Position struct {
	UserID   string   `json:"userId"`
	Callsign string   `json:"callsign"`
	Lat      float64  `json:"lat"`
	Lon      float64  `json:"lon"`
	Accuracy float64  `json:"acc,omitempty"` // meters
	Heading  *float64 `json:"hdg,omitempty"` // degrees true
	Speed    *float64 `json:"spd,omitempty"` // m/s
	TS       int64    `json:"ts"`            // unix ms of the GPS fix
}

type User struct {
	ID        string `json:"id"`
	TeamID    string `json:"teamId"`
	Callsign  string `json:"callsign"`
	FolderID  string `json:"folderId"`          // personal folder
	OwnerID   string `json:"ownerId,omitempty"` // set for agents: the user who owns them
	CreatedAt int64  `json:"createdAt"`
}

// IsAgent reports whether u is an agent (a sub-user acting for OwnerID).
func (u User) IsAgent() bool { return u.OwnerID != "" }

// PublicUser is what other team members get to see.
type PublicUser struct {
	ID       string `json:"id"`
	Callsign string `json:"callsign"`
	FolderID string `json:"folderId"`
	OwnerID  string `json:"ownerId,omitempty"` // agents only
	Revoked  bool   `json:"revoked,omitempty"` // agents only
}

func (u User) Public() PublicUser {
	return PublicUser{ID: u.ID, Callsign: u.Callsign, FolderID: u.FolderID, OwnerID: u.OwnerID}
}

type Invite struct {
	Token     string `json:"token"`
	TeamID    string `json:"teamId"`
	CreatedAt int64  `json:"createdAt"`
	ExpiresAt int64  `json:"expiresAt"` // unix ms, 0 = never
	Revoked   bool   `json:"revoked,omitempty"`
	Uses      int    `json:"uses"`
}

func (i Invite) Active(now time.Time) bool {
	return !i.Revoked && (i.ExpiresAt == 0 || now.UnixMilli() < i.ExpiresAt)
}

var (
	ErrInvalidInvite = errors.New("invalid or expired invite")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrCallsignTaken = errors.New("callsign already taken")
	ErrNotFound      = errors.New("not found")
	// ErrForeignItem means the item id already exists in another team.
	ErrForeignItem = errors.New("item belongs to another team")
	// ErrFolderInUse refuses deleting a personal or non-empty folder.
	ErrFolderInUse = errors.New("folder is personal or not empty")
	// ErrForbidden refuses an agent write outside its own folder.
	ErrForbidden = errors.New("agents may only change objects in their own folder")
)

const schema = `
CREATE TABLE IF NOT EXISTS teams (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS invites (
	token      TEXT PRIMARY KEY,
	team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL DEFAULT 0,
	revoked    INTEGER NOT NULL DEFAULT 0,
	uses       INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS users (
	id         TEXT PRIMARY KEY,
	team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	callsign   TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	created_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS users_team_callsign ON users(team_id, callsign COLLATE NOCASE);
CREATE TABLE IF NOT EXISTS items (
	id         TEXT PRIMARY KEY,
	team_id    TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	kind       TEXT NOT NULL,
	name       TEXT NOT NULL DEFAULT '',
	color      TEXT NOT NULL DEFAULT '',
	remarks    TEXT NOT NULL DEFAULT '',
	coords     TEXT NOT NULL DEFAULT '[]',
	created_by TEXT NOT NULL,
	updated_by TEXT NOT NULL,
	updated_at INTEGER NOT NULL,
	deleted    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS items_team ON items(team_id);
CREATE TABLE IF NOT EXISTS positions (
	user_id  TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
	team_id  TEXT NOT NULL,
	callsign TEXT NOT NULL,
	lat      REAL NOT NULL,
	lon      REAL NOT NULL,
	acc      REAL NOT NULL DEFAULT 0,
	hdg      REAL,
	spd      REAL,
	ts       INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS positions_team ON positions(team_id);
`

type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path.
func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// A single connection serializes writes (no SQLITE_BUSY) and is plenty here.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// --- teams & invites ---

// CreateTeam creates a team together with its (non-expiring) invite.
func (s *Store) CreateTeam(name string) (Team, Invite, error) {
	t := Team{ID: NewUUID(), Name: name, CreatedAt: time.Now().UnixMilli()}
	if _, err := s.db.Exec(`INSERT INTO teams (id, name, created_at) VALUES (?, ?, ?)`, t.ID, t.Name, t.CreatedAt); err != nil {
		return Team{}, Invite{}, err
	}
	inv, err := s.CreateInvite(t.ID, 0)
	return t, inv, err
}

func (s *Store) Teams() ([]Team, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at FROM teams ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Team{}
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) Team(id string) (Team, error) {
	var t Team
	err := s.db.QueryRow(`SELECT id, name, created_at FROM teams WHERE id = ?`, id).Scan(&t.ID, &t.Name, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Team{}, ErrNotFound
	}
	return t, err
}

func (s *Store) CreateInvite(teamID string, ttl time.Duration) (Invite, error) {
	now := time.Now()
	inv := Invite{Token: randHex(16), TeamID: teamID, CreatedAt: now.UnixMilli()}
	if ttl > 0 {
		inv.ExpiresAt = now.Add(ttl).UnixMilli()
	}
	_, err := s.db.Exec(`INSERT INTO invites (token, team_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		inv.Token, inv.TeamID, inv.CreatedAt, inv.ExpiresAt)
	if err != nil && strings.Contains(err.Error(), "FOREIGN KEY") {
		return Invite{}, ErrNotFound
	}
	return inv, err
}

// Invites lists invites, optionally only for one team ("" = all teams).
func (s *Store) Invites(teamID string) ([]Invite, error) {
	rows, err := s.db.Query(`SELECT token, team_id, created_at, expires_at, revoked, uses FROM invites
		WHERE ?1 = '' OR team_id = ?1 ORDER BY created_at`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invite{}
	for rows.Next() {
		var i Invite
		if err := rows.Scan(&i.Token, &i.TeamID, &i.CreatedAt, &i.ExpiresAt, &i.Revoked, &i.Uses); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

type queryer interface {
	QueryRow(string, ...any) *sql.Row
}

func activeInvite(q queryer, token string) (Invite, error) {
	var i Invite
	err := q.QueryRow(`SELECT token, team_id, created_at, expires_at, revoked, uses FROM invites WHERE token = ?`, token).
		Scan(&i.Token, &i.TeamID, &i.CreatedAt, &i.ExpiresAt, &i.Revoked, &i.Uses)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !i.Active(time.Now())) {
		return Invite{}, ErrInvalidInvite
	}
	return i, err
}

// InviteTeam returns the team an active invite leads to (for the join screen).
func (s *Store) InviteTeam(token string) (Team, error) {
	inv, err := activeInvite(s.db, token)
	if err != nil {
		return Team{}, err
	}
	return s.Team(inv.TeamID)
}

func (s *Store) RevokeInvite(token string) (bool, error) {
	res, err := s.db.Exec(`UPDATE invites SET revoked = 1 WHERE token = ?`, token)
	if err != nil {
		return false, err
	}
	return affected(res) > 0, nil
}

// --- users ---

// Join redeems an invite and returns the new user, their team and their
// session token. The plain token is returned only here; the store keeps its hash.
func (s *Store) Join(inviteToken, callsign string) (User, Team, string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return User{}, Team{}, "", err
	}
	defer tx.Rollback()
	inv, err := activeInvite(tx, inviteToken)
	if err != nil {
		return User{}, Team{}, "", err
	}
	token := randHex(32)
	u := User{ID: NewUUID(), TeamID: inv.TeamID, Callsign: callsign, FolderID: NewUUID(), CreatedAt: time.Now().UnixMilli()}
	_, err = tx.Exec(`INSERT INTO users (id, team_id, callsign, token_hash, created_at, folder_id) VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.TeamID, u.Callsign, hashToken(token), u.CreatedAt, u.FolderID)
	if err != nil {
		// A team shares one link, so callsigns are what tell people apart.
		if strings.Contains(err.Error(), "users.team_id, users.callsign") {
			return User{}, Team{}, "", ErrCallsignTaken
		}
		return User{}, Team{}, "", err
	}
	if err := createPersonalFolder(tx, u); err != nil {
		return User{}, Team{}, "", err
	}
	if _, err := tx.Exec(`UPDATE invites SET uses = uses + 1 WHERE token = ?`, inv.Token); err != nil {
		return User{}, Team{}, "", err
	}
	var t Team
	if err := tx.QueryRow(`SELECT id, name, created_at FROM teams WHERE id = ?`, u.TeamID).Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
		return User{}, Team{}, "", err
	}
	return u, t, token, tx.Commit()
}

func (s *Store) UserByToken(token string) (User, error) {
	var u User
	err := s.db.QueryRow(`SELECT id, team_id, callsign, folder_id, owner_id, created_at FROM users WHERE token_hash = ? AND revoked = 0`, hashToken(token)).
		Scan(&u.ID, &u.TeamID, &u.Callsign, &u.FolderID, &u.OwnerID, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	return u, err
}

// RemoveUser deletes a user (invalidating their token and dropping their
// position) and returns the team they were in. Items they created stay.
// The user's agents are revoked, not deleted, so their work keeps its author.
func (s *Store) RemoveUser(id string) (teamID string, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	err = tx.QueryRow(`DELETE FROM users WHERE id = ? RETURNING team_id`, id).Scan(&teamID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(revokeSQL+` WHERE owner_id = ?`, id); err != nil {
		return "", err
	}
	return teamID, tx.Commit()
}

func (s *Store) Users(teamID string) ([]PublicUser, error) {
	rows, err := s.db.Query(`SELECT id, callsign, folder_id, owner_id, revoked FROM users WHERE team_id = ? ORDER BY callsign`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PublicUser{}
	for rows.Next() {
		var u PublicUser
		if err := rows.Scan(&u.ID, &u.Callsign, &u.FolderID, &u.OwnerID, &u.Revoked); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// --- items ---

const itemCols = `id, kind, name, color, remarks, coords, folder, created_by, updated_by, updated_at, deleted`

func scanItem(row interface{ Scan(...any) error }, extra ...any) (Item, error) {
	var it Item
	var coords string
	dest := append([]any{&it.ID, &it.Kind, &it.Name, &it.Color, &it.Remarks, &coords, &it.Folder, &it.CreatedBy, &it.UpdatedBy, &it.UpdatedAt, &it.Deleted}, extra...)
	if err := row.Scan(dest...); err != nil {
		return Item{}, err
	}
	if err := json.Unmarshal([]byte(coords), &it.Coords); err != nil {
		return Item{}, fmt.Errorf("item %s coords: %w", it.ID, err)
	}
	if len(it.Coords) == 0 {
		it.Coords = nil
	}
	return it, nil
}

// PutItem applies an upsert or tombstone with last-write-wins on
// (UpdatedAt, UpdatedBy) on behalf of user by. It returns the item now
// stored and whether the incoming one was applied. An id that exists in
// another team is refused, as is deleting a personal or non-empty folder.
// A map object sent without a folder goes into the user's personal folder.
// Agents may only write map objects inside their own folder (ErrForbidden).
func (s *Store) PutItem(teamID string, in Item, by User) (Item, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Item{}, false, err
	}
	defer tx.Rollback()
	in.UpdatedBy = by.ID
	if in.Kind == KindFolder {
		in.Folder = ""
	} else if in.Folder == "" {
		in.Folder = by.FolderID
	}

	var curTeam string
	cur, err := scanItem(tx.QueryRow(`SELECT `+itemCols+`, team_id FROM items WHERE id = ?`, in.ID), &curTeam)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		in.CreatedBy = by.ID
	case err != nil:
		return Item{}, false, err
	case curTeam != teamID:
		return Item{}, false, ErrForeignItem
	case in.UpdatedAt < cur.UpdatedAt || (in.UpdatedAt == cur.UpdatedAt && in.UpdatedBy <= cur.UpdatedBy):
		return cur, false, nil
	default:
		in.CreatedBy = cur.CreatedBy
	}
	if by.IsAgent() {
		// Both the object as it is and as it would be must sit in the agent's folder.
		if in.Kind == KindFolder || (!in.Deleted && in.Folder != by.FolderID) ||
			(cur.ID != "" && (cur.Kind == KindFolder || cur.Folder != by.FolderID)) {
			return cur, false, ErrForbidden
		}
	}
	if in.Deleted && cur.Kind == KindFolder && !cur.Deleted {
		var users, items int
		if err := tx.QueryRow(`SELECT (SELECT count(*) FROM users WHERE folder_id = ?1),
			(SELECT count(*) FROM items WHERE folder = ?1 AND deleted = 0)`, in.ID).Scan(&users, &items); err != nil {
			return Item{}, false, err
		}
		if users > 0 || items > 0 {
			return cur, false, ErrFolderInUse
		}
	}
	if in.Deleted {
		// Keep only what is needed to replicate the tombstone.
		in = Item{ID: in.ID, Kind: in.Kind, CreatedBy: in.CreatedBy, UpdatedBy: in.UpdatedBy, UpdatedAt: in.UpdatedAt, Deleted: true}
	}
	coords, _ := json.Marshal(in.Coords)
	_, err = tx.Exec(`INSERT INTO items (id, team_id, kind, name, color, remarks, coords, folder, created_by, updated_by, updated_at, deleted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET kind = excluded.kind, name = excluded.name, color = excluded.color,
			remarks = excluded.remarks, coords = excluded.coords, folder = excluded.folder,
			updated_by = excluded.updated_by, updated_at = excluded.updated_at, deleted = excluded.deleted`,
		in.ID, teamID, in.Kind, in.Name, in.Color, in.Remarks, string(coords), in.Folder, in.CreatedBy, in.UpdatedBy, in.UpdatedAt, in.Deleted)
	if err != nil {
		return Item{}, false, err
	}
	return in, true, tx.Commit()
}

func (s *Store) Items(teamID string) ([]Item, error) {
	rows, err := s.db.Query(`SELECT `+itemCols+` FROM items WHERE team_id = ?`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// --- positions ---

// SetPosition records p as the latest fix of user u and reports whether it
// was stored. Fixes older than the stored one, or from removed users, are dropped.
func (s *Store) SetPosition(u User, p Position) (bool, error) {
	res, err := s.db.Exec(`INSERT INTO positions (user_id, team_id, callsign, lat, lon, acc, hdg, spd, ts)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM users WHERE id = ?)
		ON CONFLICT(user_id) DO UPDATE SET callsign = excluded.callsign, lat = excluded.lat, lon = excluded.lon,
			acc = excluded.acc, hdg = excluded.hdg, spd = excluded.spd, ts = excluded.ts
		WHERE excluded.ts >= positions.ts`,
		u.ID, u.TeamID, u.Callsign, p.Lat, p.Lon, p.Accuracy, p.Heading, p.Speed, p.TS, u.ID)
	if err != nil {
		return false, err
	}
	return affected(res) > 0, nil
}

func (s *Store) Positions(teamID string) ([]Position, error) {
	rows, err := s.db.Query(`SELECT user_id, callsign, lat, lon, acc, hdg, spd, ts FROM positions WHERE team_id = ?`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Position{}
	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.UserID, &p.Callsign, &p.Lat, &p.Lon, &p.Accuracy, &p.Heading, &p.Speed, &p.TS); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// --- helpers ---

// NewUUID returns a random (version 4) UUID.
func NewUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func hashToken(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

func affected(res sql.Result) int64 {
	n, _ := res.RowsAffected()
	return n
}

// Item returns one item of the team, or ErrNotFound.
func (s *Store) Item(teamID, id string) (Item, error) {
	var team string
	it, err := scanItem(s.db.QueryRow(`SELECT `+itemCols+`, team_id FROM items WHERE id = ?`, id), &team)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && team != teamID) {
		return Item{}, ErrNotFound
	}
	return it, err
}
