# sitaw

Lightweight, web-based ATAK-CIV equivalent: teams share live positions,
waypoints, lines and areas on a map. One server hosts many teams (multi-tenant). It replaces a TAK Server with a single Go
binary and a vanilla-JS PWA. The feature scope, mapped from the ATAK-CIV user
guide, is in `docs/FEATURES.md`. Keep its status column up to date when you
ship something.

## Stack and constraints

- **Backend**: Go (stdlib, `github.com/coder/websocket`, `modernc.org/sqlite`).
  Keep dependencies minimal.
- **Storage**: SQLite (`data/sitaw.db`, WAL, a single connection). The schema is in
  `internal/store/store.go` (`CREATE TABLE IF NOT EXISTS`, with no migration tool yet).
  `modernc.org/sqlite` is pinned to v1.38.2 because newer versions need a newer Go than
  1.23. Run go commands with `GOTOOLCHAIN=local` if `go get` tries to download a toolchain.
- **Multi-tenant**: every row has a `team_id`, and every store query and hub
  broadcast is scoped by team. A user belongs to exactly one team. Never add a
  query or broadcast without the team filter; `TestTeamIsolation` guards this.
- **Frontend**: vanilla JS ES modules, **no build step, no npm**. Leaflet 1.9.4
  is vendored in `web/static/vendor/` (it must work offline, so no CDNs).
  `web/static` is embedded into the binary with `go:embed`.
- **Map**: OpenTopoMap is the default base layer; OSM is the alternative. Tile layers use
  `crossOrigin: true` so the service worker can cache non-opaque responses.
- **Coordinates**: MGRS everywhere (`js/mgrs.js`, hand-written and tested by
  round-trip). All UI formatting goes through `js/coords.js` so other formats
  can be added later. Don't call `toMGRS` directly from UI code.
- **Mobile first, map-first UI**: no bars. A column of labelled tiles in the
  **top-right** (View, Go to, Objects, Draw). Opening one **hides the column**, and
  the menu takes its place: one fixed spot for menus and panels (`.pop`, `#sheet`),
  each with a header bar (title or "‹ back", plus ✕). Closing it, or tapping the
  map, brings the column back. The bottom-left corner shows callsign · team, center MGRS, GPS and
  server status (a Leaflet control, so the scale stacks above it).
  - View: base map, grid/team, per-folder show/hide. There are no per-type toggles.
  - Go to: a single search box. It accepts a position (MGRS, or lat/lon in
    decimal, DM or DMS, with or without N/S/E/W; `parseAnyCoord` in `coords.js`)
    and searches team members, objects (by name, remarks or folder name) and folders.
    Enter picks the first result. A result only moves the map; it does **not**
    open details. With an empty box it lists Me and the team.
  - Objects: folders with their objects, for browsing. Tapping an object flies
    to it and highlights it (yellow outline plus the selected row) while the menu
    **stays open**, so you can step through objects. It does **not** open the
    details panel; the ⓘ on each row does that. `focusItem` frames the object in
    the map area below the menu.
    The folders form an accordion: exactly one is unfolded, and it is the **current
    folder**, where new objects go (`prefs.drawFolder`, default: your personal
    folder; `setCurrentFolder`). Tapping another folder makes it current, which
    unfolds it and folds the previous one. The list only; map visibility is View.
  - Details and forms open in a **panel in the same place as the menus**
    (`#sheet`, `openSheet`). A panel opened from a
    menu (`openFrom('objects', …)`) has a "‹ OBJ" back button; one opened from a
    map tap or link does not. Its action row is sticky at the bottom. There is no
    bottom sheet and no `alert`/`confirm`.
  Icons are inline SVG in `js/icons.js`, so they work offline.
  Touch targets are at least 48 px, main buttons 56 px (`--tap`).
- **Offline**: the PWA shell is network-first (3 s timeout) with a cache fallback, tiles are
  cache-first (capped), and item edits go to an IndexedDB outbox that is
  replayed on reconnect.
- **Links**: every object has a URL: `/i/<item uuid>` and `/u/<user id>`.
  Selecting an object pushes the URL; closing the panel goes back to `/`. A link
  carries only the id. The object comes from the team's own replica, so a
  non-member gets the "members only" screen and a member of another team gets
  "not part of your team". The local IndexedDB cache is stamped with its team
  (`kv.team`) and wiped if it doesn't match the session's team.

## Visual design: utilitarian brutalism (decided)

Usable over nice. The rules are in the header of `web/static/css/app.css`.
Follow them for anything new:
- No rounded corners, shadows, blur, gradients or transitions. Panels are
  opaque, with visible 2px borders (`--line`).
- One font: the **system monospace** stack (`--font`), with no web font to download.
  Labels and headings are UPPERCASE. Units stay lowercase (`m`, `km`), so never
  uppercase distances. Digits are tabular.
- Light panels by default (white, black ink). One signal color (`--signal`, yellow)
  marks active/selected. Red only for danger, green only for ok/live. State is
  shown as icon + text, never color alone. Pressed = inverted (ink <-> paper).
- Main controls are square tiles with an icon **and** a text label (VIEW / GO TO /
  OBJ / DRAW; the draw fan and drawing controls are labelled too), joined into
  stacks with shared borders.
- Every panel has a header bar with an explicit ✕ close (`.sheet-x`), plus
  "‹ <menu>" back (`.sheet-back`) when opened from a menu, both added by `openSheet`.
- Colors only come from the CSS tokens. **Night mode** (`html[data-night]`,
  toggled in View, stored in prefs, applied pre-paint by an inline script in
  index.html) swaps the tokens to red-on-black, and runs the map tiles through
  the SVG `#night-red` color matrix (luminance -> 45% red, no green or blue).
  Object colors stay so they remain distinguishable.

## Layout

```
cmd/sitaw/main.go        flags, startup invite, periodic flush, graceful shutdown
internal/store/          SQLite: teams, invites, users/sessions, LWW item merge, positions
internal/store/legacy.go one-time import of the old JSON state file into team "default"
internal/server/         HTTP API, WebSocket hub and protocol (ws.go), validation
web/embed.go             embeds web/static
web/static/
  index.html, css/app.css
  sw.js                  service worker (bump VERSION when the SHELL list changes)
  js/app.js              boot, auth/join, UI wiring, menus and panels, GPS
  js/sync.js             WebSocket client, local replica, outbox, LWW
  js/db.js               IndexedDB kv/items/outbox
  js/layers.js           item, team and self rendering, length/area helpers
  js/draw.js             tap/crosshair drawing for waypoint/line/area
  js/mgrs.js             lat/lon <-> UTM <-> MGRS
  js/mgrsgrid.js         Leaflet MGRS grid overlay
  js/coords.js           coordinate format abstraction
  js/icons.js            inline SVG icon set
docs/FEATURES.md         ATAK-CIV feature map and sitaw status
```

## Folders

- Flat (folders don't nest). A folder is an item with `kind: "folder"` and no
  coords. It uses the same table, sync, outbox, LWW and `/i/<id>` links as map
  objects. Map objects carry `folder` (a folder id).
- Every user has a personal folder (`users.folder_id`), named after the callsign
  and created at join. The roster (`snapshot.users[].folderId`) tells clients
  which folders are personal.
- Anyone in the team can create, rename or move between folders (the team trusts
  itself). Delete is refused server-side (`ErrFolderInUse`) for personal or
  non-empty folders. The refusal ack carries the server copy, which the client
  force-applies.
- A map object sent without `folder` lands in the sender's personal folder.
  Clients file objects whose folder disappeared under the creator's personal folder.
- Visibility and the destination for new objects are per-device prefs
  (`localStorage sitaw.view`). By default you see only your own folder plus
  folders you created.
- `internal/store/migrate.go` upgrades older databases idempotently on every
  `Open` (adds the columns, backfills personal folders, puts orphans in "Unsorted").

## Sync model (read before touching ws.go or sync.js)

- The WebSocket protocol is documented at the top of `internal/server/ws.go`.
  Client and server must agree on it, so change both together.
- Items are `waypoint | line | area | folder`, with coords as `[lat, lon]` pairs. A delete is a
  **tombstone** (`deleted: true`), never a removal, so offline peers converge.
- Conflicts are last-write-wins on `(updatedAt, updatedBy)`. The same comparison
  exists in `store.PutItem` and `newer()` in `sync.js`, and the two must match.
- Clients stamp `updatedAt` with server-corrected time (the snapshot carries `now`).
  The server clamps timestamps more than 5 minutes in the future. The ack echoes `sentAt` so the
  client can clear its outbox entry even when the server clamped the timestamp.
- The server overwrites identity fields (`userId`, `callsign`, `createdBy`,
  `updatedBy`). Never trust them from the client.
- Positions: only the latest fix per user is kept, and fixes are not queued while offline.

## Auth

- **A team = its invite link.** An admin creates a team (`POST /api/admin/teams`),
  which returns the link. The whole team shares one link (decided). Everyone who
  opens it becomes a separate user, so callsigns must be unique within the team
  (case-insensitive, 409 otherwise). The same callsign is fine in another team.
- `GET /api/invites/<token>` (public) tells the join screen the team name.
- Invite link `/join?t=<token>`. The user picks a callsign and gets a random session
  token (stored hashed server-side, in `localStorage` client-side). The WebSocket
  authenticates with `?token=` because browsers can't set headers on it.
- Admin API (`/api/admin/teams`, `/teams/{id}/invites`, `/teams/{id}/users`,
  `DELETE /invites/{token}`, `DELETE /users/{id}`) uses `Authorization: Bearer $SITAW_ADMIN_TOKEN`.
  `DELETE /api/admin/users/{id}` kicks a user: the token is invalidated, their
  sockets are closed, and `{"t":"leave"}` is broadcast. Their items stay.
  On startup with an empty database, the server imports `data/sitaw.json` (if
  present) or creates team "default". It always logs every team's active invite links.

## Commands

```sh
go test -race ./...                       # store, team isolation, end-to-end WebSocket tests
node --test web/test/                     # JS unit tests (coords, MGRS); web/package.json only sets "type": "module"
go run ./cmd/sitaw -addr :8080            # http://localhost:8080, invite URL is in the log
go vet ./... && gofmt -l .
```

Geolocation and service workers need a secure context: use `localhost`, or
put the server behind TLS (e.g. Caddy) when testing on a phone.
`web/static` is embedded with `go:embed`, so **restart the server after editing
JS/CSS** (`go run` rebuilds). A single reload then picks up the new files.

## Known gaps

- The MGRS grid overlay ignores the Norway/Svalbard zone exceptions. Coordinate
  readouts handle them correctly.
- JS tests are thin: `web/test/*.test.mjs` cover coordinate parsing and MGRS.
  UI flows were checked by hand in headless Chrome, with no automated UI tests.
