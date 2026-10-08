package store

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// objects drops folders and positions, leaving drawn map objects.
func objects(items []Item) []Item {
	var out []Item
	for _, it := range items {
		if it.Kind != KindFolder && it.Kind != KindPosition {
			out = append(out, it)
		}
	}
	return out
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestJoinCallsignsAndRevoke(t *testing.T) {
	s := open(t)
	if _, _, _, err := s.Join("nope", "A"); err != ErrInvalidInvite {
		t.Fatalf("want ErrInvalidInvite, got %v", err)
	}
	team, inv, err := s.CreateTeam("alpha team")
	if err != nil {
		t.Fatal(err)
	}
	u, gotTeam, tok, err := s.Join(inv.Token, "Alpha")
	if err != nil || gotTeam.ID != team.ID || u.TeamID != team.ID {
		t.Fatalf("join: %v %+v", err, gotTeam)
	}
	if _, _, _, err := s.Join(inv.Token, "ALPHA"); err != ErrCallsignTaken {
		t.Fatalf("duplicate callsign (case-insensitive): %v", err)
	}
	// Same callsign is fine in another team.
	_, inv2 := must2(s.CreateTeam("other"))
	if _, _, _, err := s.Join(inv2.Token, "alpha"); err != nil {
		t.Fatalf("callsign in other team: %v", err)
	}
	if got := must(s.UserByToken(tok)); got.ID != u.ID {
		t.Fatalf("lookup: %+v", got)
	}
	if ok := must(s.RevokeInvite(inv.Token)); !ok {
		t.Fatal("revoke")
	}
	if _, _, _, err := s.Join(inv.Token, "Bravo"); err != ErrInvalidInvite {
		t.Fatalf("revoked invite accepted: %v", err)
	}
	if _, err := s.UserByToken(tok); err != nil {
		t.Fatal("existing session should survive invite revocation")
	}
	expired := must(s.CreateInvite(team.ID, time.Nanosecond))
	time.Sleep(time.Millisecond)
	if _, err := s.InviteTeam(expired.Token); err != ErrInvalidInvite {
		t.Fatalf("expired invite: %v", err)
	}
}

func must2[A, B any](a A, b B, err error) (A, B) {
	if err != nil {
		panic(err)
	}
	return a, b
}

func TestPutItemLWWAndTeams(t *testing.T) {
	s := open(t)
	ta, _ := must2(s.CreateTeam("a"))
	tb, _ := must2(s.CreateTeam("b"))
	id := NewUUID()
	base := Item{ID: id, Kind: KindWaypoint, Coords: [][2]float64{{1, 2}}}

	a := base
	a.Name, a.UpdatedAt = "first", 100
	if _, ok, err := s.PutItem(ta.ID, a, User{ID: "u1", FolderID: "f-u1"}); !ok || err != nil {
		t.Fatalf("first put: %v %v", ok, err)
	}
	old := base
	old.Name, old.UpdatedAt = "stale", 50
	if cur, ok, _ := s.PutItem(ta.ID, old, User{ID: "u2", FolderID: "f-u2"}); ok || cur.Name != "first" {
		t.Fatalf("stale put applied: %+v", cur)
	}
	tie := base
	tie.Name, tie.UpdatedAt = "tie", 100
	if _, ok, _ := s.PutItem(ta.ID, tie, User{ID: "u0", FolderID: "f-u0"}); ok {
		t.Fatal("lower updatedBy won tie")
	}
	if cur, ok, _ := s.PutItem(ta.ID, tie, User{ID: "u9", FolderID: "f-u9"}); !ok || cur.CreatedBy != "u1" {
		t.Fatalf("tie: ok=%v %+v", ok, cur)
	}

	// Another team can neither see nor overwrite it, even with a newer edit.
	hijack := base
	hijack.Name, hijack.UpdatedAt = "mine now", 999
	if _, _, err := s.PutItem(tb.ID, hijack, User{ID: "x", FolderID: "f-x"}); err != ErrForeignItem {
		t.Fatalf("cross-team put: %v", err)
	}
	if n := len(must(s.Items(tb.ID))); n != 0 {
		t.Fatalf("team b sees %d items", n)
	}
	items := must(s.Items(ta.ID))
	if len(items) != 1 || items[0].Name != "tie" || items[0].Coords[0] != [2]float64{1, 2} {
		t.Fatalf("team a items: %+v", items)
	}

	del := Item{ID: id, Kind: KindWaypoint, UpdatedAt: 200, Deleted: true}
	cur, ok, _ := s.PutItem(ta.ID, del, User{ID: "u2", FolderID: "f-u2"})
	if !ok || !cur.Deleted || cur.Name != "" || cur.Coords != nil {
		t.Fatalf("tombstone: %+v", cur)
	}
}

// setPos writes u's position object as a GPS fix.
func setPos(s *Store, team string, u User, lat, lon float64, at int64) (Item, bool, error) {
	return s.PutItem(team, Item{ID: u.PositionID, Kind: KindPosition, Coords: [][2]float64{{lat, lon}},
		Pos: &PosMeta{Source: "gps", Acc: 5, Fix: at}, UpdatedAt: at}, u)
}

func TestPositionObjects(t *testing.T) {
	s := open(t)
	ta, ia := must2(s.CreateTeam("a"))
	tb, ib := must2(s.CreateTeam("b"))
	ua, _, _, _ := s.Join(ia.Token, "A")
	ua2, _, _, _ := s.Join(ia.Token, "A2")
	ub, _, _, _ := s.Join(ib.Token, "B")
	if ua.PositionID == "" || ua.PositionID == ua2.PositionID {
		t.Fatalf("position ids: %q %q", ua.PositionID, ua2.PositionID)
	}

	it, ok, err := setPos(s, ta.ID, ua, 1, 2, 10)
	if !ok || err != nil || it.Folder != ua.FolderID || it.Name != "A" || it.Pos.Source != "gps" {
		t.Fatalf("own position: %v %v %+v", ok, err, it)
	}
	if _, ok, _ := setPos(s, ta.ID, ua, 9, 9, 5); ok {
		t.Fatal("older fix accepted")
	}
	setPos(s, tb.ID, ub, 3, 4, 10)
	pa := must(s.Positions(ta.ID))
	if len(pa) != 1 || pa[0].UserID != ua.ID || pa[0].Lat != 1 || pa[0].Callsign != "A" || pa[0].TS != 10 || pa[0].Source != "gps" {
		t.Fatalf("team a positions: %+v", pa)
	}

	// Nobody else may write it, even a teammate; nor may anyone reuse the id.
	if _, _, err := s.PutItem(ta.ID, Item{ID: ua.PositionID, Kind: KindPosition, Coords: [][2]float64{{0, 0}},
		Pos: &PosMeta{Source: "manual", Fix: 20}, UpdatedAt: 20}, ua2); err != ErrForbidden {
		t.Fatalf("teammate writing my position: %v", err)
	}
	if _, _, err := s.PutItem(ta.ID, Item{ID: ua2.PositionID, Kind: KindWaypoint, Coords: [][2]float64{{0, 0}}, UpdatedAt: 1}, ua); err != ErrForbidden {
		t.Fatalf("squatting on a position id: %v", err)
	}
	if _, _, err := s.PutItem(ta.ID, Item{ID: NewUUID(), Kind: KindPosition, Coords: [][2]float64{{0, 0}},
		Pos: &PosMeta{Source: "gps", Fix: 1}, UpdatedAt: 1}, ua); err != ErrForbidden {
		t.Fatalf("second position object: %v", err)
	}
	// Its own user may turn it into a manual one, and delete it (location unknown).
	man, ok, _ := s.PutItem(ta.ID, Item{ID: ua.PositionID, Kind: KindPosition, Coords: [][2]float64{{5, 6}},
		Pos: &PosMeta{Source: "manual", Fix: 30}, UpdatedAt: 30}, ua)
	if !ok || man.Pos.Source != "manual" {
		t.Fatalf("manual: %+v", man)
	}
	if _, ok, _ := s.PutItem(ta.ID, Item{ID: ua.PositionID, Kind: KindPosition, Deleted: true, UpdatedAt: 40}, ua); !ok {
		t.Fatal("removing own position")
	}
	if n := len(must(s.Positions(ta.ID))); n != 0 {
		t.Fatal("deleted position still listed")
	}
	if _, ok, _ := setPos(s, ta.ID, ua, 1, 2, 50); !ok {
		t.Fatal("position back after unknown")
	}

	// Removing the user removes their position (tombstone returned for broadcast).
	team, gone, err := s.RemoveUser(ua.ID)
	if err != nil || team != ta.ID || len(gone) != 1 || gone[0].ID != ua.PositionID || !gone[0].Deleted {
		t.Fatalf("remove: %v %q %+v", err, team, gone)
	}
	if n := len(must(s.Positions(ta.ID))); n != 0 {
		t.Fatal("position kept after removal")
	}
	if _, _, err := s.RemoveUser(ua.ID); err != ErrNotFound {
		t.Fatalf("second remove: %v", err)
	}
}

// The old positions table becomes position objects on upgrade.
func TestMigratePositions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.db")
	s := must(Open(path))
	team, inv := must2(s.CreateTeam("a"))
	u, _, _, _ := s.Join(inv.Token, "A")
	for _, q := range []string{
		`UPDATE users SET position_id = ''`,
		`INSERT INTO positions (user_id, team_id, callsign, lat, lon, acc, ts) VALUES ('` + u.ID + `', '` + team.ID + `', 'A', 52.1, 21.2, 7, 1234)`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	s = must(Open(path))
	defer s.Close()
	ps := must(s.Positions(team.ID))
	if len(ps) != 1 || ps[0].Lat != 52.1 || ps[0].Accuracy != 7 || ps[0].TS != 1234 || ps[0].Source != "gps" {
		t.Fatalf("migrated: %+v", ps)
	}
	var left int
	s.db.QueryRow(`SELECT count(*) FROM positions`).Scan(&left)
	if left != 0 {
		t.Fatal("old positions rows not cleared")
	}
}

func TestPersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.db")
	s := must(Open(path))
	team, inv := must2(s.CreateTeam("a"))
	_, _, tok, _ := s.Join(inv.Token, "A")
	must2(s.PutItem(team.ID, Item{ID: NewUUID(), Kind: KindWaypoint, Coords: [][2]float64{{1, 2}}, UpdatedAt: 1}, User{ID: "u", FolderID: "f-u"}))
	s.Close()

	s = must(Open(path))
	defer s.Close()
	if _, err := s.UserByToken(tok); err != nil {
		t.Fatal("token lost after reopen")
	}
	if n := len(objects(must(s.Items(team.ID)))); n != 1 {
		t.Fatalf("items after reopen: %d", n)
	}
}

func TestImportLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sitaw.json")
	legacy := `{
	 "users": {"u1": {"id": "u1", "callsign": "rs", "tokenHash": "` + hashToken("tok") + `", "createdAt": 1}},
	 "invites": {"inv1": {"token": "inv1", "createdAt": 1, "expiresAt": 0, "uses": 1}},
	 "items": {"6f1c2c1e-0000-4000-8000-000000000001": {"id": "6f1c2c1e-0000-4000-8000-000000000001", "kind": "line",
	   "name": "LN-1", "coords": [[52, 21], [52.1, 21.1]], "createdBy": "u1", "updatedBy": "u1", "updatedAt": 5}},
	 "positions": {"u1": {"userId": "u1", "callsign": "rs", "lat": 52, "lon": 21, "ts": 7}}
	}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	s := open(t)
	team, err := s.ImportLegacyJSON(path, "default")
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.UserByToken("tok")
	if err != nil || u.TeamID != team.ID {
		t.Fatalf("old session not kept: %v %+v", err, u)
	}
	if got, err := s.InviteTeam("inv1"); err != nil || got.ID != team.ID {
		t.Fatalf("old invite: %v", err)
	}
	if items := objects(must(s.Items(team.ID))); len(items) != 1 || items[0].Name != "LN-1" || len(items[0].Coords) != 2 || items[0].Folder != u.FolderID {
		t.Fatalf("items: %+v", items)
	}
	if n := len(must(s.Positions(team.ID))); n != 1 {
		t.Fatalf("positions: %d", n)
	}
	if _, err := os.Stat(path + ".imported"); err != nil {
		t.Fatal("legacy file not renamed")
	}
}

func TestNewUUID(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for range 100 {
		if u := NewUUID(); !re.MatchString(u) {
			t.Fatalf("bad uuid %q", u)
		}
	}
}

func TestFolders(t *testing.T) {
	s := open(t)
	team, inv := must2(s.CreateTeam("a"))
	u, _, _, err := s.Join(inv.Token, "Alpha")
	if err != nil || u.FolderID == "" {
		t.Fatalf("join: %v %+v", err, u)
	}
	// Joining creates a personal folder named after the callsign.
	f, err := s.Item(team.ID, u.FolderID)
	if err != nil || f.Kind != KindFolder || f.Name != "Alpha" || f.CreatedBy != u.ID {
		t.Fatalf("personal folder: %v %+v", err, f)
	}
	if users := must(s.Users(team.ID)); users[0].FolderID != u.FolderID {
		t.Fatalf("roster lacks folder: %+v", users)
	}

	// A map object sent without a folder lands in the sender's folder.
	wp, _, _ := s.PutItem(team.ID, Item{ID: NewUUID(), Kind: KindWaypoint, Coords: [][2]float64{{1, 2}}, UpdatedAt: 1}, u)
	if wp.Folder != u.FolderID {
		t.Fatalf("default folder: %q", wp.Folder)
	}

	// Shared folder: can't delete while it holds objects, can once empty.
	shared, _, _ := s.PutItem(team.ID, Item{ID: NewUUID(), Kind: KindFolder, Name: "Recon", UpdatedAt: 2, Folder: "ignored"}, u)
	if shared.Folder != "" {
		t.Fatal("folders must not live in folders")
	}
	wp.Folder, wp.UpdatedAt = shared.ID, 3
	must2(s.PutItem(team.ID, wp, u))
	if _, _, err := s.PutItem(team.ID, Item{ID: shared.ID, Kind: KindFolder, Deleted: true, UpdatedAt: 4}, u); err != ErrFolderInUse {
		t.Fatalf("delete non-empty folder: %v", err)
	}
	wp.Folder, wp.UpdatedAt = u.FolderID, 5
	must2(s.PutItem(team.ID, wp, u))
	if _, ok, err := s.PutItem(team.ID, Item{ID: shared.ID, Kind: KindFolder, Deleted: true, UpdatedAt: 6}, u); !ok || err != nil {
		t.Fatalf("delete empty folder: %v %v", ok, err)
	}
	// A personal folder can never be deleted, even when empty.
	wp.Deleted, wp.UpdatedAt = true, 7
	must2(s.PutItem(team.ID, wp, u))
	later := time.Now().UnixMilli() + 1000 // newer than the folder, so LWW alone would accept it
	if _, _, err := s.PutItem(team.ID, Item{ID: u.FolderID, Kind: KindFolder, Deleted: true, UpdatedAt: later}, u); err != ErrFolderInUse {
		t.Fatalf("delete personal folder: %v", err)
	}
}

// Databases from before folders get personal folders and filed objects.
func TestMigrateFolders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.db")
	s := must(Open(path))
	team, inv := must2(s.CreateTeam("a"))
	u, _, _, _ := s.Join(inv.Token, "A")
	// Simulate a v1 database: no folders anywhere.
	for _, q := range []string{
		`UPDATE users SET folder_id = ''`,
		`DELETE FROM items WHERE kind = 'folder'`,
		`INSERT INTO items (id, team_id, kind, coords, created_by, updated_by, updated_at) VALUES ('i1', '` + team.ID + `', 'waypoint', '[[1,2]]', '` + u.ID + `', '', 1)`,
		`INSERT INTO items (id, team_id, kind, coords, created_by, updated_by, updated_at) VALUES ('i2', '` + team.ID + `', 'waypoint', '[[1,2]]', 'gone', '', 1)`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()

	s = must(Open(path))
	defer s.Close()
	users := must(s.Users(team.ID))
	if users[0].FolderID == "" {
		t.Fatal("no personal folder after migration")
	}
	byID := map[string]Item{}
	folders := map[string]string{}
	for _, it := range must(s.Items(team.ID)) {
		byID[it.ID] = it
		if it.Kind == KindFolder {
			folders[it.ID] = it.Name
		}
	}
	if byID["i1"].Folder != users[0].FolderID {
		t.Fatalf("own object not filed: %+v", byID["i1"])
	}
	if folders[byID["i2"].Folder] != "Unsorted" {
		t.Fatalf("orphan not in Unsorted: %+v", byID["i2"])
	}
}

func TestAgents(t *testing.T) {
	s := open(t)
	team, inv := must2(s.CreateTeam("a"))
	owner, _, _, _ := s.Join(inv.Token, "probe")
	other, _, _, _ := s.Join(inv.Token, "bravo")

	a, tok, err := s.CreateAgent(owner)
	if err != nil || a.Callsign != "PROBE-ALPHA" || a.OwnerID != owner.ID || !strings.HasPrefix(tok, AgentTokenPrefix) {
		t.Fatalf("first agent: %v %+v %q", err, a, tok)
	}
	b, _, _ := s.CreateAgent(owner)
	if b.Callsign != "PROBE-BRAVO" {
		t.Fatalf("second agent: %q", b.Callsign)
	}
	// A human who already holds the next callsign makes the agent skip it.
	s.Join(inv.Token, "probe-charlie")
	c, _, _ := s.CreateAgent(owner)
	if c.Callsign != "PROBE-DELTA" {
		t.Fatalf("skip taken callsign: %q", c.Callsign)
	}
	if _, _, err := s.CreateAgent(a); err != ErrForbidden {
		t.Fatalf("agent creating an agent: %v", err)
	}

	// The token logs in as the agent; it has its own personal folder.
	got, err := s.UserByToken(tok)
	if err != nil || got.ID != a.ID || !got.IsAgent() {
		t.Fatalf("agent login: %v %+v", err, got)
	}
	if f, err := s.Item(team.ID, a.FolderID); err != nil || f.Name != "PROBE-ALPHA" {
		t.Fatalf("agent folder: %v %+v", err, f)
	}

	// Writes: only inside its own folder, in both directions.
	now := time.Now().UnixMilli()
	wp := Item{ID: NewUUID(), Kind: KindWaypoint, Coords: [][2]float64{{1, 2}}, UpdatedAt: now}
	stored, ok, err := s.PutItem(team.ID, wp, a)
	if !ok || err != nil || stored.Folder != a.FolderID || stored.CreatedBy != a.ID {
		t.Fatalf("agent create in own folder: %v %v %+v", ok, err, stored)
	}
	elsewhere := wp
	elsewhere.ID, elsewhere.Folder = NewUUID(), owner.FolderID
	if _, _, err := s.PutItem(team.ID, elsewhere, a); err != ErrForbidden {
		t.Fatalf("agent create in other folder: %v", err)
	}
	moveOut := stored
	moveOut.Folder, moveOut.UpdatedAt = owner.FolderID, now+1
	if _, _, err := s.PutItem(team.ID, moveOut, a); err != ErrForbidden {
		t.Fatalf("agent moving its object out: %v", err)
	}
	humans, _, _ := s.PutItem(team.ID, Item{ID: NewUUID(), Kind: KindWaypoint, Coords: [][2]float64{{1, 2}}, UpdatedAt: now}, other)
	edit := humans
	edit.Name, edit.UpdatedAt, edit.Folder = "pwned", now+2, a.FolderID
	if _, _, err := s.PutItem(team.ID, edit, a); err != ErrForbidden {
		t.Fatalf("agent pulling a human object into its folder: %v", err)
	}
	if _, _, err := s.PutItem(team.ID, Item{ID: NewUUID(), Kind: KindFolder, Name: "x", UpdatedAt: now}, a); err != ErrForbidden {
		t.Fatalf("agent creating a folder: %v", err)
	}
	if _, ok, err := s.PutItem(team.ID, Item{ID: stored.ID, Kind: KindWaypoint, Deleted: true, UpdatedAt: now + 3}, a); !ok || err != nil {
		t.Fatalf("agent deleting its own object: %v %v", ok, err)
	}
	// Humans may still work in the agent's folder (the team trusts itself).
	if _, ok, err := s.PutItem(team.ID, Item{ID: NewUUID(), Kind: KindWaypoint, Coords: [][2]float64{{1, 2}}, Folder: a.FolderID, UpdatedAt: now}, other); !ok || err != nil {
		t.Fatalf("human writing into agent folder: %v %v", ok, err)
	}

	// Listing, revoking, and the owner's removal.
	if list := must(s.Agents(owner.ID)); len(list) != 3 {
		t.Fatalf("agents: %+v", list)
	}
	if _, _, err := s.RevokeAgent(other.ID, a.ID); err != ErrNotFound {
		t.Fatalf("revoking someone else's agent: %v", err)
	}
	if u, _, err := s.RevokeAgent(owner.ID, a.ID); err != nil || !u.Revoked {
		t.Fatalf("revoke: %v %+v", err, u)
	}
	if _, err := s.UserByToken(tok); err != ErrUnauthorized {
		t.Fatalf("revoked token still works: %v", err)
	}
	if _, _, err := s.RemoveUser(owner.ID); err != nil {
		t.Fatal(err)
	}
	for _, ag := range must(s.Agents(owner.ID)) {
		if !ag.Revoked {
			t.Fatalf("agent of removed owner still active: %+v", ag)
		}
	}
	roster := map[string]PublicUser{}
	for _, u := range must(s.Users(team.ID)) {
		roster[u.Callsign] = u
	}
	if r := roster["PROBE-ALPHA"]; !r.Revoked || r.OwnerID != owner.ID {
		t.Fatalf("revoked agent should stay in roster: %+v", r)
	}
}
