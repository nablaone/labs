package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("conflict")
	ErrForbidden = errors.New("forbidden")
	ErrLimit     = errors.New("limit reached")
)

const schema = `
CREATE TABLE IF NOT EXISTS devices (
	id         INTEGER PRIMARY KEY,
	token_hash BLOB NOT NULL UNIQUE,
	kill_hash  BLOB,
	push_url   TEXT,
	created    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS devices_kill ON devices(kill_hash);
CREATE TABLE IF NOT EXISTS invites (
	hash      BLOB PRIMARY KEY,
	device_id INTEGER REFERENCES devices(id) ON DELETE CASCADE, -- NULL: created by admin
	expires   INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS mailboxes (
	id        BLOB PRIMARY KEY,
	device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
	send_hash BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS messages (
	seq       INTEGER PRIMARY KEY AUTOINCREMENT,
	id        BLOB NOT NULL UNIQUE,
	mailbox   BLOB NOT NULL REFERENCES mailboxes(id) ON DELETE CASCADE,
	device_id INTEGER NOT NULL,
	ts        INTEGER NOT NULL,
	body      BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS messages_dev ON messages(device_id, seq);
CREATE INDEX IF NOT EXISTS messages_ts ON messages(ts);
CREATE TABLE IF NOT EXISTS blobs (
	id            BLOB PRIMARY KEY,
	device_id     INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
	max_downloads INTEGER NOT NULL,
	downloads     INTEGER NOT NULL DEFAULT 0,
	expires       INTEGER NOT NULL
);
`

// Store is the server's SQLite state. It holds only routing data and
// undelivered ciphertext.
type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=secure_delete(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite: one writer; keeps things simple
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func hash(b []byte) []byte { h := sha256.Sum256(b); return h[:] }

func randBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

func now() int64 { return time.Now().UnixMilli() }

// ---- devices ----

func (s *Store) CreateDevice(ctx context.Context, invite, tokenHash, killHash []byte, pushURL string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM invites WHERE hash = ? AND expires > ?`, hash(invite), now())
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrForbidden
	}
	_, err = tx.Exec(`INSERT INTO devices(token_hash, kill_hash, push_url, created) VALUES (?, ?, ?, ?)`,
		tokenHash, killHash, nullStr(pushURL), now())
	if isUnique(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Device returns the device ID for a bearer token.
func (s *Store) Device(ctx context.Context, token []byte) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM devices WHERE token_hash = ?`, hash(token)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

func (s *Store) SetPushURL(ctx context.Context, dev int64, url string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE devices SET push_url = ? WHERE id = ?`, nullStr(url), dev)
	return err
}

func (s *Store) PushURL(ctx context.Context, dev int64) (string, error) {
	var u sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT push_url FROM devices WHERE id = ?`, dev).Scan(&u)
	return u.String, err
}

// DeleteDevice removes the device and, via cascade, its mailboxes, queued
// messages, invites and blob records. It returns blob IDs whose files the
// caller must remove.
func (s *Store) DeleteDevice(ctx context.Context, dev int64) ([][]byte, error) {
	blobs, err := s.collect(ctx, `SELECT id FROM blobs WHERE device_id = ?`, dev)
	if err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM devices WHERE id = ?`, dev)
	return blobs, err
}

// DeviceByKill finds a device by kill-code hash.
func (s *Store) DeviceByKill(ctx context.Context, killHash []byte) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM devices WHERE kill_hash = ?`, killHash).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// ---- invites ----

// CreateInvite makes a single-use invite. dev == 0 means admin (no limit).
func (s *Store) CreateInvite(ctx context.Context, dev int64, maxOpen int, ttl time.Duration) ([]byte, time.Time, error) {
	var owner any
	if dev != 0 {
		owner = dev
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM invites WHERE device_id = ? AND expires > ?`, dev, now()).Scan(&n); err != nil {
			return nil, time.Time{}, err
		}
		if n >= maxOpen {
			return nil, time.Time{}, ErrLimit
		}
	}
	inv := randBytes(16)
	exp := time.Now().Add(ttl)
	_, err := s.db.ExecContext(ctx, `INSERT INTO invites(hash, device_id, expires) VALUES (?, ?, ?)`, hash(inv), owner, exp.UnixMilli())
	return inv, exp, err
}

// ---- mailboxes ----

func (s *Store) CreateMailbox(ctx context.Context, dev int64, id, sendHash []byte) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO mailboxes(id, device_id, send_hash) VALUES (?, ?, ?)`, id, dev, sendHash)
	if isUnique(err) {
		return ErrConflict
	}
	return err
}

func (s *Store) DeleteMailbox(ctx context.Context, dev int64, id []byte) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM mailboxes WHERE id = ? AND device_id = ?`, id, dev)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Mailbox returns owner device and send-token hash.
func (s *Store) Mailbox(ctx context.Context, id []byte) (dev int64, sendHash []byte, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT device_id, send_hash FROM mailboxes WHERE id = ?`, id).Scan(&dev, &sendHash)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return
}

// ---- messages ----

type Message struct {
	Seq     int64
	ID      []byte
	Mailbox []byte
	TS      int64
	Body    []byte
}

func (s *Store) Enqueue(ctx context.Context, dev int64, mailbox, body []byte) ([]byte, error) {
	id := randBytes(16)
	_, err := s.db.ExecContext(ctx, `INSERT INTO messages(id, mailbox, device_id, ts, body) VALUES (?, ?, ?, ?, ?)`,
		id, mailbox, dev, now(), body)
	return id, err
}

// Pending returns queued messages for a device with seq > after.
func (s *Store) Pending(ctx context.Context, dev, after int64, limit int) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT seq, id, mailbox, ts, body FROM messages WHERE device_id = ? AND seq > ? ORDER BY seq LIMIT ?`, dev, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.Seq, &m.ID, &m.Mailbox, &m.TS, &m.Body); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Ack deletes delivered messages belonging to dev.
func (s *Store) Ack(ctx context.Context, dev int64, ids [][]byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM messages WHERE id = ? AND device_id = ?`, id, dev); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---- blobs ----

func (s *Store) CreateBlob(ctx context.Context, dev int64, id []byte, maxDownloads int, ttl time.Duration) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO blobs(id, device_id, max_downloads, expires) VALUES (?, ?, ?, ?)`,
		id, dev, maxDownloads, time.Now().Add(ttl).UnixMilli())
	return err
}

func (s *Store) BlobExists(ctx context.Context, id []byte) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM blobs WHERE id = ? AND expires > ?`, id, now()).Scan(&n)
	return n == 1, err
}

// BlobDownloaded counts a completed download and reports whether the blob is
// now used up (record deleted; caller removes the file).
func (s *Store) BlobDownloaded(ctx context.Context, id []byte) (done bool, err error) {
	if _, err = s.db.ExecContext(ctx, `UPDATE blobs SET downloads = downloads + 1 WHERE id = ?`, id); err != nil {
		return
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM blobs WHERE id = ? AND downloads >= max_downloads`, id)
	if err != nil {
		return
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// Expire removes old messages, invites and blobs; returns expired blob IDs.
func (s *Store) Expire(ctx context.Context, msgTTL time.Duration) ([][]byte, error) {
	t := now()
	if _, err := s.db.ExecContext(ctx, `DELETE FROM messages WHERE ts < ?`, t-msgTTL.Milliseconds()); err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM invites WHERE expires <= ?`, t); err != nil {
		return nil, err
	}
	ids, err := s.collect(ctx, `SELECT id FROM blobs WHERE expires <= ?`, t)
	if err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM blobs WHERE expires <= ?`, t)
	return ids, err
}

// AllBlobIDs lists live blob records (used to clean orphan files at start).
func (s *Store) AllBlobIDs(ctx context.Context) ([][]byte, error) {
	return s.collect(ctx, `SELECT id FROM blobs`)
}

func (s *Store) collect(ctx context.Context, q string, args ...any) ([][]byte, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]byte
	for rows.Next() {
		var id []byte
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func isUnique(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "PRIMARY KEY"))
}
