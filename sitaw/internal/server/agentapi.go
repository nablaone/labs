package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"sitaw/internal/geo"
	"sitaw/internal/store"
)

// Agent API (/api/v1): REST for AI agents, authenticated with an agent
// token (or any user token). Everything is scoped to the caller's team.
// Agents read all team data; the store confines their writes to their own
// folder. Coordinates are accepted as MGRS or lat/lon text and returned as
// both. GET /agent (agent.md) documents this for the agent.

const staleAfter = 5 * time.Minute

// --- auth + rate limit ---

type rateLimiter struct {
	mu   sync.Mutex
	wins map[string]*window
}

type window struct {
	start         time.Time
	reads, writes int
}

const (
	readsPerMin  = 120
	writesPerMin = 30
)

// allow counts one request for user and reports whether it is within limits.
func (l *rateLimiter) allow(user string, write bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	w := l.wins[user]
	if w == nil || now.Sub(w.start) >= time.Minute {
		w = &window{start: now}
		l.wins[user] = w
	}
	if write {
		w.writes++
		return w.writes <= writesPerMin
	}
	w.reads++
	return w.reads <= readsPerMin
}

func (s *Server) withAPIUser(h func(http.ResponseWriter, *http.Request, store.User)) http.HandlerFunc {
	return s.withUser(func(w http.ResponseWriter, r *http.Request, u store.User) {
		write := r.Method != http.MethodGet
		if !s.limiter.allow(u.ID, write) {
			w.Header().Set("Retry-After", "60")
			httpError(w, http.StatusTooManyRequests, fmt.Sprintf("rate limit: %d reads and %d writes per minute", readsPerMin, writesPerMin))
			return
		}
		if u.IsAgent() {
			_ = s.store.TouchUser(u.ID)
		}
		h(w, r, u)
	})
}

// publicURL is the address clients use to reach sitaw: the configured
// external address (SITAW_BASE_URL) if set, otherwise derived from the request
// (X-Forwarded-Proto/Host from a proxy, else Host). Set it when behind a proxy
// or tunnel that doesn't forward those headers, e.g. tailscale serve.
func (s *Server) publicURL(r *http.Request) string {
	if s.baseURL != "" {
		return s.baseURL
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
		if r.TLS != nil {
			proto = "https"
		}
	}
	return proto + "://" + host
}

// isMapObject: drawn objects (waypoint, line, area). Folders and positions
// have their own endpoints.
func isMapObject(it store.Item) bool {
	return it.Kind != store.KindFolder && it.Kind != store.KindPosition
}

// --- views ---

type pointView struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	MGRS string  `json:"mgrs,omitempty"`
}

func point(lat, lon float64) pointView {
	m, _ := geo.ToMGRS(lat, lon, 5)
	return pointView{Lat: round6(lat), Lon: round6(lon), MGRS: m}
}

func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }

type objectView struct {
	ID         string      `json:"id"`
	Kind       string      `json:"kind"`
	Name       string      `json:"name"`
	Color      string      `json:"color,omitempty"`
	Remarks    string      `json:"remarks,omitempty"`
	FolderID   string      `json:"folderId"`
	FolderName string      `json:"folderName"`
	Points     []pointView `json:"points"`
	Center     pointView   `json:"center"`
	LengthM    *float64    `json:"lengthM,omitempty"`
	AreaM2     *float64    `json:"areaM2,omitempty"`
	CreatedBy  string      `json:"createdBy"` // callsign
	ByAgent    bool        `json:"byAgent,omitempty"`
	UpdatedAt  string      `json:"updatedAt"`
	Link       string      `json:"link"`
	DistanceM  *float64    `json:"distanceM,omitempty"` // set by near/nearby queries
}

type folderView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Personal string `json:"personalOf,omitempty"` // callsign, for personal folders
	Agent    bool   `json:"agentFolder,omitempty"`
	Objects  int    `json:"objects"`
	Yours    bool   `json:"yours,omitempty"` // the caller's own folder (agents may only write here)
}

type memberView struct {
	ID       string   `json:"id"`
	Callsign string   `json:"callsign"`
	Agent    bool     `json:"agent,omitempty"`
	Owner    string   `json:"owner,omitempty"` // owner callsign, for agents
	Revoked  bool     `json:"revoked,omitempty"`
	FolderID string   `json:"folderId"`
	Position *posView `json:"position,omitempty"`
}

type posView struct {
	pointView
	AccuracyM  float64  `json:"accuracyM,omitempty"`
	HeadingDeg *float64 `json:"headingDeg,omitempty"`
	SpeedMS    *float64 `json:"speedMs,omitempty"`
	Time       string   `json:"time"`
	AgeSeconds int64    `json:"ageSeconds"`
	Stale      bool     `json:"stale"`
}

// teamData is one consistent read of everything the API needs.
type teamData struct {
	team      store.Team
	users     map[string]store.PublicUser
	positions map[string]store.Position
	items     []store.Item
	byID      map[string]store.Item
	folders   map[string]store.Item
	base      string
}

func (s *Server) loadTeam(r *http.Request, teamID string) (*teamData, error) {
	t, err := s.store.Team(teamID)
	if err != nil {
		return nil, err
	}
	users, err := s.store.Users(teamID)
	if err != nil {
		return nil, err
	}
	pos, err := s.store.Positions(teamID)
	if err != nil {
		return nil, err
	}
	items, err := s.store.Items(teamID)
	if err != nil {
		return nil, err
	}
	d := &teamData{team: t, users: map[string]store.PublicUser{}, positions: map[string]store.Position{},
		byID: map[string]store.Item{}, folders: map[string]store.Item{}, base: s.publicURL(r)}
	for _, u := range users {
		d.users[u.ID] = u
	}
	for _, p := range pos {
		d.positions[p.UserID] = p
	}
	for _, it := range items {
		if it.Deleted {
			continue
		}
		d.items = append(d.items, it)
		d.byID[it.ID] = it
		if it.Kind == store.KindFolder {
			d.folders[it.ID] = it
		}
	}
	return d, nil
}

// folderOf mirrors the client: objects whose folder is gone show under their
// creator's personal folder.
func (d *teamData) folderOf(it store.Item) string {
	if _, ok := d.folders[it.Folder]; ok {
		return it.Folder
	}
	if u, ok := d.users[it.CreatedBy]; ok {
		return u.FolderID
	}
	return it.Folder
}

func (d *teamData) callsign(id string) string {
	if u, ok := d.users[id]; ok {
		return u.Callsign
	}
	return "unknown"
}

func (d *teamData) object(it store.Item) objectView {
	fid := d.folderOf(it)
	v := objectView{ID: it.ID, Kind: it.Kind, Name: it.Name, Color: it.Color, Remarks: it.Remarks,
		FolderID: fid, FolderName: d.folders[fid].Name, CreatedBy: d.callsign(it.CreatedBy),
		ByAgent: d.users[it.CreatedBy].OwnerID != "", UpdatedAt: time.UnixMilli(it.UpdatedAt).UTC().Format(time.RFC3339),
		Link: d.base + "/i/" + it.ID}
	for _, c := range it.Coords {
		v.Points = append(v.Points, point(c[0], c[1]))
	}
	lat, lon := center(it.Coords)
	v.Center = point(lat, lon)
	switch it.Kind {
	case store.KindLine:
		l := pathLength(it.Coords)
		v.LengthM = &l
	case store.KindArea:
		ar := polygonArea(it.Coords)
		v.AreaM2 = &ar
	}
	return v
}

func center(c [][2]float64) (float64, float64) {
	if len(c) == 0 {
		return 0, 0
	}
	var lat, lon float64
	for _, p := range c {
		lat += p[0]
		lon += p[1]
	}
	return lat / float64(len(c)), lon / float64(len(c))
}

func pathLength(c [][2]float64) float64 {
	d := 0.0
	for i := 1; i < len(c); i++ {
		d += geo.Distance(c[i-1][0], c[i-1][1], c[i][0], c[i][1])
	}
	return math.Round(d)
}

// polygonArea matches polygonArea in layers.js (spherical excess).
func polygonArea(c [][2]float64) float64 {
	const R, rad = 6378137.0, math.Pi / 180
	s := 0.0
	for i := range c {
		p1, p2 := c[i], c[(i+1)%len(c)]
		s += (p2[1] - p1[1]) * rad * (2 + math.Sin(p1[0]*rad) + math.Sin(p2[0]*rad))
	}
	return math.Round(math.Abs(s * R * R / 2))
}

// distanceTo is how far a point is from an object (0 inside an area).
func distanceTo(lat, lon float64, it store.Item) float64 {
	switch it.Kind {
	case store.KindArea:
		if geo.InPolygon(lat, lon, it.Coords) {
			return 0
		}
		return geo.DistanceToPath(lat, lon, it.Coords, true)
	default:
		return geo.DistanceToPath(lat, lon, it.Coords, false)
	}
}

func (d *teamData) member(u store.PublicUser, now time.Time) memberView {
	m := memberView{ID: u.ID, Callsign: u.Callsign, Agent: u.OwnerID != "", Revoked: u.Revoked, FolderID: u.FolderID}
	if m.Agent {
		m.Owner = d.callsign(u.OwnerID)
	}
	if p, ok := d.positions[u.ID]; ok {
		pv := d.position(p, now)
		m.Position = &pv
	}
	return m
}

func (d *teamData) position(p store.Position, now time.Time) posView {
	age := now.Sub(time.UnixMilli(p.TS))
	return posView{pointView: point(p.Lat, p.Lon), AccuracyM: p.Accuracy, HeadingDeg: p.Heading, SpeedMS: p.Speed,
		Time: time.UnixMilli(p.TS).UTC().Format(time.RFC3339), AgeSeconds: int64(age.Seconds()), Stale: age > staleAfter}
}

// resolvePoint turns "at" values into a position: a coordinate (MGRS or
// degrees), an object id (its center), or a team member's callsign.
func (d *teamData) resolvePoint(s string) (float64, float64, error) {
	s = strings.TrimSpace(s)
	if p, ok := geo.Parse(s); ok {
		return p.Lat, p.Lon, nil
	}
	if it, ok := d.byID[s]; ok && isMapObject(it) {
		lat, lon := center(it.Coords)
		return lat, lon, nil
	}
	for _, u := range d.users {
		if strings.EqualFold(u.Callsign, s) {
			if p, ok := d.positions[u.ID]; ok {
				return p.Lat, p.Lon, nil
			}
			return 0, 0, fmt.Errorf("%s has no position yet", u.Callsign)
		}
	}
	return 0, 0, fmt.Errorf("cannot resolve %q: use MGRS, \"lat, lon\", an object id or a callsign", s)
}

// --- read handlers ---

func (s *Server) apiTeam(w http.ResponseWriter, r *http.Request, u store.User) (*teamData, bool) {
	d, err := s.loadTeam(r, u.TeamID)
	if err != nil {
		s.internal(w, err)
		return nil, false
	}
	return d, true
}

func (s *Server) handleAPIMe(w http.ResponseWriter, r *http.Request, u store.User) {
	d, ok := s.apiTeam(w, r, u)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"you": d.member(d.users[u.ID], time.Now()), "team": d.team.Name, "yourFolder": folderView{
			ID: u.FolderID, Name: d.folders[u.FolderID].Name, Yours: true},
		"canWrite": "only objects in yourFolder (agents); humans: everything",
	})
}

func (s *Server) handleAPITeam(w http.ResponseWriter, r *http.Request, u store.User) {
	d, ok := s.apiTeam(w, r, u)
	if !ok {
		return
	}
	now := time.Now()
	members := []memberView{}
	for _, m := range d.users {
		members = append(members, d.member(m, now))
	}
	sort.Slice(members, func(i, j int) bool { return members[i].Callsign < members[j].Callsign })
	writeJSON(w, http.StatusOK, map[string]any{"team": d.team.Name, "members": members, "time": now.UTC().Format(time.RFC3339)})
}

func (s *Server) handleAPIPositions(w http.ResponseWriter, r *http.Request, u store.User) {
	d, ok := s.apiTeam(w, r, u)
	if !ok {
		return
	}
	now := time.Now()
	type row struct {
		Callsign string `json:"callsign"`
		UserID   string `json:"userId"`
		posView
	}
	out := []row{}
	for id, p := range d.positions {
		out = append(out, row{Callsign: d.callsign(id), UserID: id, posView: d.position(p, now)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AgeSeconds < out[j].AgeSeconds })
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAPIFolders(w http.ResponseWriter, r *http.Request, u store.User) {
	d, ok := s.apiTeam(w, r, u)
	if !ok {
		return
	}
	counts := map[string]int{}
	for _, it := range d.items {
		if isMapObject(it) {
			counts[d.folderOf(it)]++
		}
	}
	owners := map[string]store.PublicUser{}
	for _, m := range d.users {
		owners[m.FolderID] = m
	}
	out := []folderView{}
	for id, f := range d.folders {
		v := folderView{ID: id, Name: f.Name, Objects: counts[id], Yours: id == u.FolderID}
		if o, ok := owners[id]; ok {
			v.Personal, v.Agent = o.Callsign, o.OwnerID != ""
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAPIObjects(w http.ResponseWriter, r *http.Request, u store.User) {
	d, ok := s.apiTeam(w, r, u)
	if !ok {
		return
	}
	q := r.URL.Query()
	folder, kind, text := q.Get("folder"), q.Get("kind"), strings.ToLower(q.Get("q"))
	limit := 200
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 && n <= 1000 {
		limit = n
	}
	var bbox []float64
	if b := q.Get("bbox"); b != "" {
		for _, f := range strings.Split(b, ",") {
			v, err := strconv.ParseFloat(strings.TrimSpace(f), 64)
			if err != nil {
				httpError(w, http.StatusBadRequest, "bbox must be south,west,north,east in degrees")
				return
			}
			bbox = append(bbox, v)
		}
		if len(bbox) != 4 {
			httpError(w, http.StatusBadRequest, "bbox must be south,west,north,east in degrees")
			return
		}
	}
	var nearLat, nearLon, radius float64
	near := q.Get("near") != ""
	if near {
		var err error
		if nearLat, nearLon, err = d.resolvePoint(q.Get("near")); err != nil {
			httpError(w, http.StatusBadRequest, err.Error())
			return
		}
		radius = 1000
		if v, err := strconv.ParseFloat(q.Get("radius"), 64); err == nil && v > 0 {
			radius = v
		}
	}

	out := []objectView{}
	for _, it := range d.items {
		if !isMapObject(it) || (kind != "" && it.Kind != kind) {
			continue
		}
		fid := d.folderOf(it)
		if folder != "" && fid != folder && !strings.EqualFold(d.folders[fid].Name, folder) {
			continue
		}
		if text != "" && !strings.Contains(strings.ToLower(it.Name+" "+it.Remarks), text) {
			continue
		}
		if bbox != nil && !inBBox(it.Coords, bbox) {
			continue
		}
		v := d.object(it)
		if near {
			dist := math.Round(distanceTo(nearLat, nearLon, it))
			if dist > radius {
				continue
			}
			v.DistanceM = &dist
		}
		out = append(out, v)
	}
	if near {
		sort.Slice(out, func(i, j int) bool { return *out[i].DistanceM < *out[j].DistanceM })
	} else {
		sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	}
	if len(out) > limit {
		out = out[:limit]
	}
	writeJSON(w, http.StatusOK, out)
}

func inBBox(c [][2]float64, b []float64) bool {
	for _, p := range c {
		if p[0] >= b[0] && p[0] <= b[2] && p[1] >= b[1] && p[1] <= b[3] {
			return true
		}
	}
	return false
}

func (s *Server) handleAPIObject(w http.ResponseWriter, r *http.Request, u store.User) {
	d, ok := s.apiTeam(w, r, u)
	if !ok {
		return
	}
	it, found := d.byID[r.PathValue("id")]
	if !found || !isMapObject(it) {
		httpError(w, http.StatusNotFound, "no such object in your team")
		return
	}
	writeJSON(w, http.StatusOK, d.object(it))
}

// --- geo handlers ---

func (s *Server) handleGeoConvert(w http.ResponseWriter, r *http.Request, _ store.User) {
	p, ok := geo.Parse(r.URL.Query().Get("q"))
	if !ok {
		httpError(w, http.StatusBadRequest, `cannot parse q: use MGRS ("34U DC 12345 67890") or degrees ("52.2297, 21.0122", "52°13'47\"N 21°0'44\"E")`)
		return
	}
	pv := point(p.Lat, p.Lon)
	writeJSON(w, http.StatusOK, map[string]any{"lat": pv.Lat, "lon": pv.Lon, "mgrs": pv.MGRS,
		"precisionM": p.Precision, "inputKind": p.Kind, "note": noteFor(p)})
}

func noteFor(p geo.Point) string {
	if p.Kind == "mgrs" && p.Precision > 1 {
		return fmt.Sprintf("MGRS reference is a %g m square; lat/lon is its south-west corner", p.Precision)
	}
	return ""
}

func (s *Server) handleGeoMeasure(w http.ResponseWriter, r *http.Request, u store.User) {
	d, ok := s.apiTeam(w, r, u)
	if !ok {
		return
	}
	lat1, lon1, err := d.resolvePoint(r.URL.Query().Get("from"))
	if err != nil {
		httpError(w, http.StatusBadRequest, "from: "+err.Error())
		return
	}
	lat2, lon2, err := d.resolvePoint(r.URL.Query().Get("to"))
	if err != nil {
		httpError(w, http.StatusBadRequest, "to: "+err.Error())
		return
	}
	trueB := geo.Bearing(lat1, lon1, lat2, lon2)
	grid := math.Mod(trueB-geo.GridConvergence(lat1, lon1)+360, 360)
	writeJSON(w, http.StatusOK, map[string]any{
		"from": point(lat1, lon1), "to": point(lat2, lon2),
		"distanceM":       math.Round(geo.Distance(lat1, lon1, lat2, lon2)),
		"bearingTrueDeg":  math.Round(trueB*10) / 10,
		"bearingGridDeg":  math.Round(grid*10) / 10,
		"bearingGridMils": math.Round(grid * 6400 / 360),
	})
}

func (s *Server) handleGeoNearby(w http.ResponseWriter, r *http.Request, u store.User) {
	d, ok := s.apiTeam(w, r, u)
	if !ok {
		return
	}
	lat, lon, err := d.resolvePoint(r.URL.Query().Get("at"))
	if err != nil {
		httpError(w, http.StatusBadRequest, "at: "+err.Error())
		return
	}
	radius := 1000.0
	if v, err := strconv.ParseFloat(r.URL.Query().Get("radius"), 64); err == nil && v > 0 {
		radius = v
	}
	now := time.Now()
	type personNear struct {
		memberView
		DistanceM float64 `json:"distanceM"`
	}
	people := []personNear{}
	for id, p := range d.positions {
		if dist := math.Round(geo.Distance(lat, lon, p.Lat, p.Lon)); dist <= radius {
			people = append(people, personNear{d.member(d.users[id], now), dist})
		}
	}
	sort.Slice(people, func(i, j int) bool { return people[i].DistanceM < people[j].DistanceM })
	objects := []objectView{}
	for _, it := range d.items {
		if !isMapObject(it) {
			continue
		}
		if dist := math.Round(distanceTo(lat, lon, it)); dist <= radius {
			v := d.object(it)
			v.DistanceM = &dist
			objects = append(objects, v)
		}
	}
	sort.Slice(objects, func(i, j int) bool { return *objects[i].DistanceM < *objects[j].DistanceM })
	writeJSON(w, http.StatusOK, map[string]any{"at": point(lat, lon), "radiusM": radius, "people": people, "objects": objects})
}

func (s *Server) handleGeoInside(w http.ResponseWriter, r *http.Request, u store.User) {
	d, ok := s.apiTeam(w, r, u)
	if !ok {
		return
	}
	lat, lon, err := d.resolvePoint(r.URL.Query().Get("at"))
	if err != nil {
		httpError(w, http.StatusBadRequest, "at: "+err.Error())
		return
	}
	areas := []objectView{}
	for _, it := range d.items {
		if it.Kind == store.KindArea && geo.InPolygon(lat, lon, it.Coords) {
			areas = append(areas, d.object(it))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"at": point(lat, lon), "areas": areas})
}

// --- write handlers ---

// objectInput is what agents send. Points may be [lat, lon] pairs or
// coordinate strings (MGRS or degrees).
type objectInput struct {
	Kind    *string           `json:"kind"`
	Name    *string           `json:"name"`
	Color   *string           `json:"color"`
	Remarks *string           `json:"remarks"`
	Points  []json.RawMessage `json:"points"`
	Folder  *string           `json:"folderId"`
	Pos     *store.PosMeta    `json:"pos"` // positions: source, acc, hdg, spd, fix
}

func parsePoints(raw []json.RawMessage) ([][2]float64, error) {
	out := make([][2]float64, 0, len(raw))
	for i, r := range raw {
		var pair [2]float64
		if err := json.Unmarshal(r, &pair); err == nil {
			out = append(out, pair)
			continue
		}
		var s string
		if err := json.Unmarshal(r, &s); err != nil {
			return nil, fmt.Errorf("points[%d]: use [lat, lon] or a coordinate string", i)
		}
		p, ok := geo.Parse(s)
		if !ok {
			return nil, fmt.Errorf("points[%d]: cannot parse %q", i, s)
		}
		out = append(out, [2]float64{p.Lat, p.Lon})
	}
	return out, nil
}

var defaultColor = map[string]string{store.KindWaypoint: "#e53935", store.KindLine: "#1e88e5", store.KindArea: "#fb8c00"}

func (s *Server) handleAPICreate(w http.ResponseWriter, r *http.Request, u store.User) {
	var in objectInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&in); err != nil {
		httpError(w, http.StatusBadRequest, "bad json: "+err.Error())
		return
	}
	if in.Kind == nil {
		httpError(w, http.StatusBadRequest, "kind is required: waypoint, line or area")
		return
	}
	coords, err := parsePoints(in.Points)
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	it := store.Item{ID: store.NewUUID(), Kind: *in.Kind, Coords: coords, UpdatedAt: time.Now().UnixMilli()}
	if it.Kind == store.KindPosition {
		// Your position has a fixed id; "creating" it sets it (manual by default).
		it.ID = u.PositionID
		if cur, err := s.store.Item(u.TeamID, u.PositionID); err == nil {
			it.UpdatedAt = max(it.UpdatedAt, cur.UpdatedAt+1)
		}
		it.Pos = in.Pos
		if it.Pos == nil {
			it.Pos = &store.PosMeta{Source: "manual"}
		}
		if it.Pos.Fix == 0 {
			it.Pos.Fix = time.Now().UnixMilli()
		}
	}
	if in.Name != nil {
		it.Name = *in.Name
	}
	if in.Remarks != nil {
		it.Remarks = *in.Remarks
	}
	it.Color = defaultColor[it.Kind]
	if in.Color != nil {
		it.Color = *in.Color
	}
	if in.Folder != nil {
		it.Folder = *in.Folder
	}
	s.apiPut(w, r, u, it, http.StatusCreated)
}

func (s *Server) handleAPIUpdate(w http.ResponseWriter, r *http.Request, u store.User) {
	cur, err := s.store.Item(u.TeamID, r.PathValue("id"))
	if err != nil || cur.Deleted || cur.Kind == store.KindFolder {
		httpError(w, http.StatusNotFound, "no such object in your team")
		return
	}
	var in objectInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&in); err != nil {
		httpError(w, http.StatusBadRequest, "bad json: "+err.Error())
		return
	}
	if in.Kind != nil && *in.Kind != cur.Kind {
		httpError(w, http.StatusBadRequest, "kind cannot change; delete and create instead")
		return
	}
	it := cur
	if it.Kind == store.KindPosition && in.Points != nil && in.Pos == nil {
		it.Pos = &store.PosMeta{Source: "manual", Fix: time.Now().UnixMilli()} // moved by hand
	}
	if in.Name != nil {
		it.Name = *in.Name
	}
	if in.Remarks != nil {
		it.Remarks = *in.Remarks
	}
	if in.Color != nil {
		it.Color = *in.Color
	}
	if in.Folder != nil {
		it.Folder = *in.Folder
	}
	if in.Pos != nil {
		it.Pos = in.Pos
	}
	if in.Points != nil {
		if it.Coords, err = parsePoints(in.Points); err != nil {
			httpError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	it.UpdatedAt = max(time.Now().UnixMilli(), cur.UpdatedAt+1)
	s.apiPut(w, r, u, it, http.StatusOK)
}

func (s *Server) handleAPIDelete(w http.ResponseWriter, r *http.Request, u store.User) {
	cur, err := s.store.Item(u.TeamID, r.PathValue("id"))
	if err != nil || cur.Deleted || cur.Kind == store.KindFolder {
		httpError(w, http.StatusNotFound, "no such object in your team")
		return
	}
	tomb := store.Item{ID: cur.ID, Kind: cur.Kind, Deleted: true, UpdatedAt: max(time.Now().UnixMilli(), cur.UpdatedAt+1)}
	if _, ok := s.putAndBroadcast(w, u, tomb); ok {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) apiPut(w http.ResponseWriter, r *http.Request, u store.User, it store.Item, code int) {
	if err := validateItem(&it); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	stored, ok := s.putAndBroadcast(w, u, it)
	if !ok {
		return
	}
	d, ok := s.apiTeam(w, r, u)
	if !ok {
		return
	}
	writeJSON(w, code, d.object(stored))
}

// putAndBroadcast stores a change through the same path as the WebSocket and
// pushes it live to the team. It writes the error response itself.
func (s *Server) putAndBroadcast(w http.ResponseWriter, u store.User, it store.Item) (store.Item, bool) {
	stored, applied, err := s.store.PutItem(u.TeamID, it, u)
	switch {
	case errors.Is(err, store.ErrForbidden):
		httpError(w, http.StatusForbidden, err.Error()+" (yours: "+u.FolderID+")")
		return stored, false
	case errors.Is(err, store.ErrForeignItem):
		httpError(w, http.StatusConflict, "id in use")
		return stored, false
	case err != nil:
		s.internal(w, err)
		return stored, false
	case !applied:
		httpError(w, http.StatusConflict, "a newer version exists; fetch the object and retry")
		return stored, false
	}
	s.hub.broadcast(u.TeamID, msg{T: "item", Item: &stored})
	return stored, true
}
