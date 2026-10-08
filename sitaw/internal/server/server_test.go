package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"sitaw/internal/store"
)

func setup(t *testing.T) (*httptest.Server, *store.Store, string) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	static := fstest.MapFS{"index.html": {Data: []byte("<html>")}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := New(st, static, "admintok", "http://x", log)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	_, inv, err := st.CreateTeam("alpha")
	if err != nil {
		t.Fatal(err)
	}
	return ts, st, inv.Token
}

func join(t *testing.T, ts *httptest.Server, invite, callsign string) (string, string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"invite": invite, "callsign": callsign})
	r, err := http.Post(ts.URL+"/api/join", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatalf("join status %d", r.StatusCode)
	}
	var out struct {
		Token string           `json:"token"`
		User  store.PublicUser `json:"user"`
	}
	json.NewDecoder(r.Body).Decode(&out)
	return out.Token, out.User.ID
}

func dial(t *testing.T, ts *httptest.Server, token string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, strings.Replace(ts.URL, "http", "ws", 1)+"/ws?token="+token, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() })
	return c
}

func read(t *testing.T, c *websocket.Conn, typ string) msg {
	t.Helper()
	return readNot(t, c, typ)
}

// readNot waits for a message of type typ and fails on any of forbid first.
func readNot(t *testing.T, c *websocket.Conn, typ string, forbid ...string) msg {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, b, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for %q: %v", typ, err)
		}
		var m msg
		json.Unmarshal(b, &m)
		if m.T == typ {
			return m
		}
		for _, f := range forbid {
			if m.T == f {
				t.Fatalf("unexpected %q while waiting for %q: %s", f, typ, b)
			}
		}
	}
}

// objects counts non-folder items.
func objects(items []store.Item) int {
	n := 0
	for _, it := range items {
		if it.Kind != store.KindFolder {
			n++
		}
	}
	return n
}

func write(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	b, _ := json.Marshal(v)
	if err := c.Write(context.Background(), websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
}

func TestJoinRejectsBadInvite(t *testing.T) {
	ts, _, _ := setup(t)
	r, _ := http.Post(ts.URL+"/api/join", "application/json", strings.NewReader(`{"invite":"bad","callsign":"A"}`))
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("status %d", r.StatusCode)
	}
}

func TestWSRequiresToken(t *testing.T) {
	ts, _, _ := setup(t)
	r, _ := http.Get(ts.URL + "/ws?token=nope")
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", r.StatusCode)
	}
}

func TestSyncBroadcast(t *testing.T) {
	ts, _, invite := setup(t)
	tokA, idA := join(t, ts, invite, "ALPHA")
	tokB, _ := join(t, ts, invite, "BRAVO")

	a := dial(t, ts, tokA)
	snap := read(t, a, "snapshot")
	if snap.You == nil || snap.You.Callsign != "ALPHA" || snap.Now == 0 || snap.Team == nil || snap.Team.Name != "alpha" {
		t.Fatalf("bad snapshot: %+v", snap)
	}
	b := dial(t, ts, tokB)
	read(t, b, "snapshot")

	// Position from A reaches B, stamped with A's identity.
	write(t, a, map[string]any{"t": "pos", "pos": map[string]any{"lat": 52.1, "lon": 21.0, "ts": time.Now().UnixMilli(), "userId": "spoofed"}})
	p := read(t, b, "pos")
	if p.Pos.UserID != idA || p.Pos.Callsign != "ALPHA" {
		t.Fatalf("pos: %+v", p.Pos)
	}

	// Item from A is acked to A and broadcast to B.
	item := map[string]any{"id": "6f1c2c1e-0000-4000-8000-000000000001", "kind": "line", "name": "L1", "coords": [][2]float64{{52, 21}, {52.1, 21.1}}, "updatedAt": 1000}
	write(t, a, map[string]any{"t": "put", "item": item})
	ack := read(t, a, "ack")
	if ack.OK == nil || !*ack.OK || ack.SentAt != 1000 {
		t.Fatalf("ack: %+v", ack)
	}
	got := read(t, b, "item")
	if got.Item.Name != "L1" || got.Item.CreatedBy != idA {
		t.Fatalf("item: %+v", got.Item)
	}

	// An older edit from B loses and B gets the server copy back.
	item["name"], item["updatedAt"] = "old", 500
	write(t, b, map[string]any{"t": "put", "item": item})
	ack = read(t, b, "ack")
	if *ack.OK || ack.Item.Name != "L1" {
		t.Fatalf("stale ack: %+v", ack)
	}

	// Invalid geometry is rejected with an error.
	write(t, b, map[string]any{"t": "put", "item": map[string]any{"id": "6f1c2c1e-0000-4000-8000-000000000002", "kind": "area", "coords": [][2]float64{{1, 1}}, "updatedAt": 1}})
	ack = read(t, b, "ack")
	if ack.Error == "" {
		t.Fatalf("expected error ack: %+v", ack)
	}

	// A late joiner sees everything in the snapshot.
	tokC, _ := join(t, ts, invite, "CHARLIE")
	c := dial(t, ts, tokC)
	snap = read(t, c, "snapshot")
	if objects(snap.Items) != 1 || len(snap.Items) != 4 || len(snap.Positions) != 1 || len(snap.Users) != 3 {
		t.Fatalf("late snapshot: items=%d positions=%d users=%d", len(snap.Items), len(snap.Positions), len(snap.Users))
	}
}

func admin(t *testing.T, ts *httptest.Server, method, path, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer admintok")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAdminTeams(t *testing.T) {
	ts, _, _ := setup(t)
	r, _ := http.Post(ts.URL+"/api/admin/teams", "application/json", strings.NewReader(`{"name":"x"}`))
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no auth: %d", r.StatusCode)
	}
	r = admin(t, ts, "POST", "/api/admin/teams", `{"name":"bravo"}`)
	var out struct {
		Team store.Team `json:"team"`
		URL  string     `json:"url"`
	}
	json.NewDecoder(r.Body).Decode(&out)
	if r.StatusCode != 200 || out.Team.Name != "bravo" || !strings.HasPrefix(out.URL, "http://x/join?t=") {
		t.Fatalf("create team: %d %+v", r.StatusCode, out)
	}
	// The join screen can show which team a link leads to.
	token := strings.TrimPrefix(out.URL, "http://x/join?t=")
	r, _ = http.Get(ts.URL + "/api/invites/" + token)
	var info struct {
		Team struct{ Name string } `json:"team"`
	}
	json.NewDecoder(r.Body).Decode(&info)
	if info.Team.Name != "bravo" {
		t.Fatalf("invite info: %+v", info)
	}
	r = admin(t, ts, "GET", "/api/admin/teams", "")
	var teams []struct {
		Name    string `json:"name"`
		Invites []any  `json:"invites"`
	}
	json.NewDecoder(r.Body).Decode(&teams)
	if len(teams) != 2 || len(teams[1].Invites) != 1 {
		t.Fatalf("list teams: %+v", teams)
	}
	if r := admin(t, ts, "POST", "/api/admin/teams/nope/invites", ""); r.StatusCode != http.StatusNotFound {
		t.Fatalf("invite for missing team: %d", r.StatusCode)
	}
}

func TestDeepLinksServeApp(t *testing.T) {
	ts, _, _ := setup(t)
	for _, p := range []string{"/", "/join?t=x", "/i/6f1c2c1e-0000-4000-8000-000000000001", "/u/whatever"} {
		r, _ := http.Get(ts.URL + p)
		b, _ := io.ReadAll(r.Body)
		if r.StatusCode != 200 || string(b) != "<html>" {
			t.Fatalf("%s: %d %q", p, r.StatusCode, b)
		}
	}
}

// Two teams on one server must never see each other's data.
func TestTeamIsolation(t *testing.T) {
	ts, st, inviteA := setup(t)
	_, invB, _ := st.CreateTeam("bravo")
	tokA, _ := join(t, ts, inviteA, "ALPHA")
	tokB, _ := join(t, ts, invB.Token, "BRAVO")

	a := dial(t, ts, tokA)
	read(t, a, "snapshot")
	b := dial(t, ts, tokB)
	read(t, b, "snapshot")

	id := "6f1c2c1e-0000-4000-8000-0000000000aa"
	write(t, a, map[string]any{"t": "put", "item": map[string]any{"id": id, "kind": "waypoint", "name": "SECRET",
		"coords": [][2]float64{{52, 21}}, "updatedAt": 1000}})
	read(t, a, "ack")
	write(t, a, map[string]any{"t": "pos", "pos": map[string]any{"lat": 52, "lon": 21, "ts": time.Now().UnixMilli()}})
	read(t, a, "pos")

	// B tries to overwrite A's item by id: refused, and A's copy is untouched.
	write(t, b, map[string]any{"t": "put", "item": map[string]any{"id": id, "kind": "waypoint", "name": "PWNED",
		"coords": [][2]float64{{0, 0}}, "updatedAt": 9999}})
	ack := readNot(t, b, "ack", "item", "pos")
	if *ack.OK || ack.Item != nil {
		t.Fatalf("cross-team put: %+v", ack)
	}

	// B never got A's item or position: the first pos B sees is its own.
	write(t, b, map[string]any{"t": "pos", "pos": map[string]any{"lat": 1, "lon": 1, "ts": time.Now().UnixMilli()}})
	if m := readNot(t, b, "pos", "item"); m.Pos.Callsign != "BRAVO" {
		t.Fatalf("B received A's position: %+v", m.Pos)
	}

	// A fresh snapshot for B is still empty of A's data.
	b2 := dial(t, ts, tokB)
	snap := read(t, b2, "snapshot")
	if objects(snap.Items) != 0 || len(snap.Items) != 1 || len(snap.Users) != 1 || len(snap.Positions) != 1 || snap.Team.Name != "bravo" {
		t.Fatalf("B snapshot leaks: items=%d users=%d positions=%d", len(snap.Items), len(snap.Users), len(snap.Positions))
	}
	a2 := dial(t, ts, tokA)
	snap = read(t, a2, "snapshot")
	var secret *store.Item
	for i := range snap.Items {
		if snap.Items[i].ID == id {
			secret = &snap.Items[i]
		}
	}
	if secret == nil || secret.Name != "SECRET" {
		t.Fatalf("A's item changed: %+v", snap.Items)
	}
}

func TestSharedLinkAndKick(t *testing.T) {
	ts, _, invite := setup(t)
	tokA, idA := join(t, ts, invite, "ALPHA")
	tokB, _ := join(t, ts, invite, "BRAVO")

	// Same link, same callsign (any case): rejected with 409.
	r, _ := http.Post(ts.URL+"/api/join", "application/json", strings.NewReader(`{"invite":"`+invite+`","callsign":"alpha"}`))
	if r.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate callsign: status %d", r.StatusCode)
	}

	a := dial(t, ts, tokA)
	read(t, a, "snapshot")
	b := dial(t, ts, tokB)
	read(t, b, "snapshot")

	r = admin(t, ts, "DELETE", "/api/admin/users/"+idA, "")
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("kick: status %d", r.StatusCode)
	}

	// Others are told to drop the marker.
	if m := read(t, b, "leave"); m.UserID != idA {
		t.Fatalf("leave: %+v", m)
	}
	// The kicked connection is closed.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		if _, _, err := a.Read(ctx); err != nil {
			break
		}
	}
	if ctx.Err() != nil {
		t.Fatal("kicked connection was not closed")
	}
	// And the token no longer works.
	req, _ := http.NewRequest("GET", ts.URL+"/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+tokA)
	r, _ = http.DefaultClient.Do(req)
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("kicked token: status %d", r.StatusCode)
	}
}

func TestFoldersOverWS(t *testing.T) {
	ts, _, invite := setup(t)
	tokA, _ := join(t, ts, invite, "ALPHA")
	a := dial(t, ts, tokA)
	snap := read(t, a, "snapshot")
	myFolder := snap.Users[0].FolderID

	// Someone joining is announced together with their personal folder.
	join(t, ts, invite, "BRAVO")
	if u := read(t, a, "user"); u.User.Callsign != "BRAVO" || u.User.FolderID == "" {
		t.Fatalf("user announce: %+v", u.User)
	}
	if f := read(t, a, "item"); f.Item.Kind != "folder" || f.Item.Name != "BRAVO" {
		t.Fatalf("folder announce: %+v", f.Item)
	}

	// Deleting a personal folder is refused and the folder is handed back.
	write(t, a, map[string]any{"t": "put", "item": map[string]any{"id": myFolder, "kind": "folder", "deleted": true, "updatedAt": time.Now().UnixMilli() + 1000}})
	ack := read(t, a, "ack")
	if *ack.OK || ack.Error == "" || ack.Item == nil || ack.Item.Deleted {
		t.Fatalf("personal folder delete: %+v", ack)
	}
}
