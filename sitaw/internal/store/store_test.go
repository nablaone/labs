package store

import (
	"os"
	"path/filepath"
	"regexp"
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

// objects drops folders, leaving map objects.
func objects(items []Item) []Item {
	var out []Item
	for _, it := range items {
		if it.Kind != KindFolder {
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

func TestPositionsAndRemoveUser(t *testing.T) {
	s := open(t)
	ta, ia := must2(s.CreateTeam("a"))
	_, ib := must2(s.CreateTeam("b"))
	ua, _, _, _ := s.Join(ia.Token, "A")
	ub, _, _, _ := s.Join(ib.Token, "B")

	if ok := must(s.SetPosition(ua, Position{Lat: 1, Lon: 2, TS: 10})); !ok {
		t.Fatal("position rejected")
	}
	if ok := must(s.SetPosition(ua, Position{Lat: 9, Lon: 9, TS: 5})); ok {
		t.Fatal("older fix accepted")
	}
	must(s.SetPosition(ub, Position{Lat: 3, Lon: 4, TS: 10}))
	pa := must(s.Positions(ta.ID))
	if len(pa) != 1 || pa[0].UserID != ua.ID || pa[0].Lat != 1 {
		t.Fatalf("team a positions: %+v", pa)
	}

	if team := must(s.RemoveUser(ua.ID)); team != ta.ID {
		t.Fatalf("removed from %q", team)
	}
	if n := len(must(s.Positions(ta.ID))); n != 0 {
		t.Fatal("position kept after removal")
	}
	if ok := must(s.SetPosition(ua, Position{Lat: 1, Lon: 2, TS: 20})); ok {
		t.Fatal("removed user's position stored")
	}
	if _, err := s.RemoveUser(ua.ID); err != ErrNotFound {
		t.Fatalf("second remove: %v", err)
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
