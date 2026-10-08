package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sitaw/internal/store"
)

func do(t *testing.T, ts *httptest.Server, method, path, token, body string) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, ts.URL+path, r)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

func decode[T any](t *testing.T, b []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}
	return v
}

type createdAgent struct {
	Agent  store.Agent `json:"agent"`
	Token  string      `json:"token"`
	Prompt string      `json:"prompt"`
}

func TestAgentAPI(t *testing.T) {
	ts, st, invite := setup(t)
	tokProbe, _ := join(t, ts, invite, "probe")
	tokBravo, idBravo := join(t, ts, invite, "BRAVO")

	// A human's map connection, to see agent writes arrive live.
	human := dial(t, ts, tokBravo)
	read(t, human, "snapshot")

	// Create an agent (session auth). The prompt carries token + skill curl.
	code, b := do(t, ts, "POST", "/api/agents", tokProbe, "")
	if code != 200 {
		t.Fatalf("create agent: %d %s", code, b)
	}
	ag := decode[createdAgent](t, b)
	if ag.Agent.Callsign != "PROBE-ALPHA" || !strings.HasPrefix(ag.Token, store.AgentTokenPrefix) {
		t.Fatalf("agent: %+v", ag)
	}
	// The prompt is minimal: identity, token, and where to fetch the rest.
	wantPrompt := "You are PROBE-ALPHA, an AI agent for probe on the sitaw team map.\n" +
		"Your access token: " + ag.Token + "\n" +
		"Fetch your instructions and follow them:\n" +
		`curl -fsSL -H "Authorization: Bearer ` + ag.Token + `" ` + ts.URL + "/agent\n"
	if ag.Prompt != wantPrompt {
		t.Fatalf("prompt:\n%s\nwant:\n%s", ag.Prompt, wantPrompt)
	}
	if u := read(t, human, "user"); u.User.Callsign != "PROBE-ALPHA" || u.User.OwnerID == "" {
		t.Fatalf("team not told about agent: %+v", u.User)
	}
	read(t, human, "item") // its folder

	// Agents can't manage agents; second agent gets the next letter.
	if code, _ := do(t, ts, "POST", "/api/agents", ag.Token, ""); code != http.StatusForbidden {
		t.Fatalf("agent creating agent: %d", code)
	}
	_, b = do(t, ts, "POST", "/api/agents", tokProbe, "")
	if decode[createdAgent](t, b).Agent.Callsign != "PROBE-BRAVO" {
		t.Fatalf("second agent: %s", b)
	}

	// Reads: everything in the team, coordinates as lat/lon + MGRS.
	write(t, human, map[string]any{"t": "pos", "pos": map[string]any{"lat": 52.2297, "lon": 21.0122, "ts": time.Now().UnixMilli()}})
	read(t, human, "pos")
	write(t, human, map[string]any{"t": "put", "item": map[string]any{"id": "6f1c2c1e-0000-4000-8000-0000000000b1", "kind": "area", "name": "AO-1",
		"coords": [][2]float64{{52.22, 21.00}, {52.22, 21.03}, {52.24, 21.03}, {52.24, 21.00}}, "updatedAt": time.Now().UnixMilli()}})
	read(t, human, "ack")

	code, b = do(t, ts, "GET", "/api/v1/me", ag.Token, "")
	me := decode[map[string]any](t, b)
	if code != 200 || me["you"].(map[string]any)["callsign"] != "PROBE-ALPHA" || me["team"] != "alpha" {
		t.Fatalf("me: %d %s", code, b)
	}
	_, b = do(t, ts, "GET", "/api/v1/positions", ag.Token, "")
	pos := decode[[]map[string]any](t, b)
	if len(pos) != 1 || pos[0]["callsign"] != "BRAVO" || !strings.HasPrefix(pos[0]["mgrs"].(string), "34U EC") || pos[0]["stale"] != false {
		t.Fatalf("positions: %s", b)
	}
	_, b = do(t, ts, "GET", "/api/v1/objects?kind=area", ag.Token, "")
	objs := decode[[]map[string]any](t, b)
	if len(objs) != 1 || objs[0]["name"] != "AO-1" || objs[0]["createdBy"] != "BRAVO" || objs[0]["areaM2"].(float64) < 1e6 {
		t.Fatalf("objects: %s", b)
	}

	// Writes: create in own folder (point as MGRS text) -> live on the map.
	code, b = do(t, ts, "POST", "/api/v1/objects", ag.Token,
		`{"kind":"waypoint","name":"HOSP-1","points":["34U DC 99000 86000"],"remarks":"source: test"}`)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, b)
	}
	wp := decode[map[string]any](t, b)
	if wp["folderName"] != "PROBE-ALPHA" || wp["byAgent"] != true || wp["createdBy"] != "PROBE-ALPHA" || !strings.Contains(wp["link"].(string), "/i/") {
		t.Fatalf("created: %s", b)
	}
	// Earlier broadcasts (AO-1, PROBE-BRAVO's folder) may still be queued.
	for it := read(t, human, "item"); it.Item.Name != "HOSP-1"; it = read(t, human, "item") {
	}
	id := wp["id"].(string)
	if code, b := do(t, ts, "PATCH", "/api/v1/objects/"+id, ag.Token, `{"name":"HOSP-BANACHA","points":["52.2096, 20.9860"]}`); code != 200 ||
		decode[map[string]any](t, b)["name"] != "HOSP-BANACHA" {
		t.Fatalf("patch: %d %s", code, b)
	}

	// Writes outside its folder are refused.
	_, b = do(t, ts, "GET", "/api/v1/folders", ag.Token, "")
	var bravoFolder string
	for _, f := range decode[[]map[string]any](t, b) {
		if f["personalOf"] == "BRAVO" {
			bravoFolder = f["id"].(string)
		}
	}
	if code, _ := do(t, ts, "POST", "/api/v1/objects", ag.Token, `{"kind":"waypoint","points":["52.2,21.0"],"folderId":"`+bravoFolder+`"}`); code != http.StatusForbidden {
		t.Fatalf("create in other folder: %d", code)
	}
	if code, _ := do(t, ts, "PATCH", "/api/v1/objects/6f1c2c1e-0000-4000-8000-0000000000b1", ag.Token, `{"name":"x"}`); code != http.StatusForbidden {
		t.Fatalf("edit human object: %d", code)
	}
	if code, _ := do(t, ts, "DELETE", "/api/v1/objects/6f1c2c1e-0000-4000-8000-0000000000b1", ag.Token, ""); code != http.StatusForbidden {
		t.Fatalf("delete human object: %d", code)
	}
	if code, _ := do(t, ts, "PATCH", "/api/v1/objects/"+id, ag.Token, `{"folderId":"`+bravoFolder+`"}`); code != http.StatusForbidden {
		t.Fatalf("move out of own folder: %d", code)
	}
	if code, b := do(t, ts, "POST", "/api/v1/objects", ag.Token, `{"kind":"area","points":["52,21"]}`); code != 400 {
		t.Fatalf("invalid geometry: %d %s", code, b)
	}

	// Geo helpers, with callsigns and object ids as positions.
	_, b = do(t, ts, "GET", "/api/v1/geo/convert?q=52%C2%B013'47%22N+21%C2%B00'44%22E", ag.Token, "")
	if c := decode[map[string]any](t, b); !strings.HasPrefix(c["mgrs"].(string), "34U EC 0") {
		t.Fatalf("convert: %s", b)
	}
	_, b = do(t, ts, "GET", "/api/v1/geo/measure?from=BRAVO&to="+id, ag.Token, "")
	m := decode[map[string]any](t, b)
	if d := m["distanceM"].(float64); d < 2000 || d > 3500 {
		t.Fatalf("measure: %s", b)
	}
	_, b = do(t, ts, "GET", "/api/v1/geo/inside?at=BRAVO", ag.Token, "")
	if in := decode[map[string]any](t, b); len(in["areas"].([]any)) != 1 {
		t.Fatalf("inside: %s", b)
	}
	_, b = do(t, ts, "GET", "/api/v1/geo/nearby?at=BRAVO&radius=5000", ag.Token, "")
	nb := decode[map[string]any](t, b)
	if len(nb["people"].([]any)) != 1 || len(nb["objects"].([]any)) != 2 {
		t.Fatalf("nearby: %s", b)
	}
	if code, _ := do(t, ts, "GET", "/api/v1/geo/measure?from=nowhere&to=BRAVO", ag.Token, ""); code != 400 {
		t.Fatalf("bad point: %d", code)
	}

	// Delete own object.
	if code, _ := do(t, ts, "DELETE", "/api/v1/objects/"+id, ag.Token, ""); code != http.StatusNoContent {
		t.Fatalf("delete own: %d", code)
	}

	// Another team's agent sees none of this.
	_, invB, _ := st.CreateTeam("other")
	tokOther, _ := join(t, ts, invB.Token, "charlie")
	_, b = do(t, ts, "POST", "/api/agents", tokOther, "")
	other := decode[createdAgent](t, b)
	_, b = do(t, ts, "GET", "/api/v1/objects", other.Token, "")
	if n := len(decode[[]map[string]any](t, b)); n != 0 {
		t.Fatalf("other team's agent sees %d objects", n)
	}
	if code, _ := do(t, ts, "GET", "/api/v1/objects/6f1c2c1e-0000-4000-8000-0000000000b1", other.Token, ""); code != 404 {
		t.Fatalf("cross-team read: %d", code)
	}
	if code, _ := do(t, ts, "GET", "/api/v1/geo/measure?from=BRAVO&to=52,21", other.Token, ""); code != 400 {
		t.Fatalf("cross-team callsign lookup: %d", code)
	}

	// Listing and revoking (owner only).
	_, b = do(t, ts, "GET", "/api/agents", tokProbe, "")
	if n := len(decode[[]store.Agent](t, b)); n != 2 {
		t.Fatalf("list agents: %s", b)
	}
	if code, _ := do(t, ts, "DELETE", "/api/agents/"+ag.Agent.ID, tokBravo, ""); code != 404 {
		t.Fatalf("revoking someone else's agent: %d", code)
	}
	if code, _ := do(t, ts, "DELETE", "/api/agents/"+ag.Agent.ID, tokProbe, ""); code != http.StatusNoContent {
		t.Fatalf("revoke: %d", code)
	}
	if code, _ := do(t, ts, "GET", "/api/v1/me", ag.Token, ""); code != 401 {
		t.Fatalf("revoked token: %d", code)
	}
	for u := read(t, human, "user"); !u.User.Revoked; u = read(t, human, "user") {
		if u.User.ID == ag.Agent.ID {
			t.Fatalf("revocation not broadcast: %+v", u.User)
		}
	}
	_ = idBravo
}

func TestAgentRateLimitAndSkill(t *testing.T) {
	ts, _, invite := setup(t)
	tok, _ := join(t, ts, invite, "probe")
	_, b := do(t, ts, "POST", "/api/agents", tok, "")
	ag := decode[createdAgent](t, b)
	limited := false
	for i := 0; i < readsPerMin+5; i++ {
		if code, _ := do(t, ts, "GET", "/api/v1/me", ag.Token, ""); code == http.StatusTooManyRequests {
			limited = i >= readsPerMin
			break
		}
	}
	if !limited {
		t.Fatal("rate limit not enforced at the right count")
	}

	// Instructions need the token and are personalised.
	if res, _ := http.Get(ts.URL + "/agent"); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("instructions without token: %d", res.StatusCode)
	}
	_, b = do(t, ts, "POST", "/api/agents", tok, "") // fresh agent: the first one is rate-limited now
	fresh := decode[createdAgent](t, b)
	code, body := do(t, ts, "GET", "/agent", fresh.Token, "")
	if code != 200 {
		t.Fatalf("instructions: %d", code)
	}
	for _, want := range []string{"You are PROBE-BRAVO", "agent of **probe**", `team **"alpha"**`, fresh.Agent.FolderID,
		ts.URL + "/api/v1", "/geo/measure", "untrusted", "SALUTE"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Fatalf("instructions lack %q", want)
		}
	}
	if bytes.Contains(body, []byte("{{")) || bytes.Contains(body, []byte(fresh.Token)) {
		t.Fatal("unrendered template, or the token echoed back, in instructions")
	}
}
