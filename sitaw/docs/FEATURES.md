# sitaw — feature list

Source: ATAK Civilian 4.0 Software User Manual (TAK Product Center, 2020),
sections listed in its table of contents. Each ATAK-CIV feature is mapped to a
sitaw scope:

- **MVP**: in the first working version
- **Next**: planned, after the MVP is stable
- **Later**: maybe, low priority
- **No**: out of scope for a lightweight web client

Status: `[x]` done, `[~]` scaffolded / partial, `[ ]` not started.
The backend `[x]` items are covered by Go tests. The frontend `[x]` items are
implemented but have not been tested by hand in a browser yet.

## 1. Map display

| ATAK-CIV feature | sitaw scope | Status | Notes |
|---|---|---|---|
| Moving map, pan / pinch zoom | MVP | [x] | Leaflet |
| Online base maps | MVP | [x] | OpenTopoMap is the default; OSM is the alternative |
| Map layer switching ("Maps & Favorites") | MVP | [x] | Layer control |
| Offline maps (cached tiles) | MVP | [~] | Service worker caches tiles you view; "cache this area" button is Next |
| Bookmark a location (favorites) | Next | [ ] | |
| North-up / track-up, manual rotation | Later | [ ] | Leaflet has no native rotation |
| Center on self, lock to self | MVP | [x] | "Me" button, follow mode |
| Map scale | MVP | [x] | |
| Coordinate display of map center / cursor | MVP | [x] | MGRS |
| 3D view, 3D models (DTED) | No | | |
| Rubber sheet (georeference an image) | Later | [ ] | |

## 2. Coordinates and grid

| ATAK-CIV feature | sitaw scope | Status | Notes |
|---|---|---|---|
| MGRS coordinates everywhere | MVP | [x] | Default and, for now, the only format |
| MGRS grid overlay | MVP | [x] | 100 km / 10 km / 1 km lines, chosen by zoom |
| Enter coordinate / MGRS location | MVP | [~] | Go to search accepts MGRS and lat/lon (decimal, DM, DMS); placing a point by typed coordinate is Next |
| Other formats (lat/lon DD, DMS, UTM) | Next | [~] | Input: DD/DM/DMS accepted in Go to. Display is still MGRS only (`coords.js`) |
| Go To tool | MVP | [x] | MGRS input |

## 3. Self marker and team presence (SA)

| ATAK-CIV feature | sitaw scope | Status | Notes |
|---|---|---|---|
| Self-marker from GPS | MVP | [x] | `navigator.geolocation.watchPosition` |
| Send own position to the team | MVP | [x] | Throttled, over WebSocket |
| Show other users' positions | MVP | [x] | Colored circle and callsign label |
| Stale / no-GPS indicator | MVP | [x] | Markers fade when a position is old |
| Callsign | MVP | [x] | Set when joining |
| Team color | MVP | [~] | One team for now, one color |
| Roles (Team Lead, HQ, Medic, ...) | Next | [ ] | Letter inside the circle |
| GPS error circle | MVP | [x] | Accuracy circle |
| Tracking breadcrumbs (own and others) | Next | [ ] | Server keeps a short track history |
| Track history / track search | Later | [ ] | |
| Multiple teams | MVP | [x] | Multi-tenant: a team is created with its invite link; data is isolated per team |

## 4. Placement (points)

| ATAK-CIV feature | sitaw scope | Status | Notes |
|---|---|---|---|
| Point Dropper: drop a marker by tapping | MVP | [x] | Waypoint |
| Affiliations: Unknown / Neutral / Hostile / Friendly | Next | [ ] | Simple colored shapes, not full MIL-STD-2525 |
| Custom name prefix and auto-index | Next | [ ] | Default names are `WP-1`, `WP-2`, ... |
| Recently added list | MVP | [x] | Objects menu: folders with their objects |
| Details: name, remarks, color, coordinate | MVP | [x] | Edit sheet |
| Elevation of a point | Later | [ ] | Would need a DEM |
| Attachments, images (Quick Pic, Gallery) | Later | [ ] | |
| Send / broadcast marker | MVP | [x] | Every item is broadcast to the team automatically |
| Auto send (periodic re-broadcast) | No | | Server state and sync on reconnect replace this |
| Custom iconsets | Later | [ ] | |
| Red X (inspect a point's coordinates) | Next | [ ] | Plan: long-press shows the MGRS of that point |

## 5. Drawing tools

| ATAK-CIV feature | sitaw scope | Status | Notes |
|---|---|---|---|
| Polyline (open free form) | MVP | [x] | "Line" |
| Polygon (closed free form) | MVP | [x] | "Area" |
| Circle (center and radius, rings) | Next | [ ] | |
| Rectangle (3-point, tactical overlay) | Next | [ ] | |
| Telestrate (finger freehand) | Later | [ ] | |
| Color, opacity, line thickness | MVP | [~] | Color is supported; width and opacity are Next |
| Edit vertices, move or delete a shape | Next | [ ] | Delete and rename work in the MVP |
| Labels (length, area) | Next | [ ] | |
| Geofence (entry / exit alerts on a shape) | Later | [ ] | |

## 6. Measurement and navigation

| ATAK-CIV feature | sitaw scope | Status | Notes |
|---|---|---|---|
| Range & Bearing line | Next | [ ] | Distance and azimuth between two points |
| R&B circle / range rings | Next | [ ] | |
| Bullseye | Later | [ ] | |
| Routes (create, checkpoints, KML/GPX import and export) | Later | [ ] | Lines cover the MVP need |
| Navigate-to (Quick Nav, Bloodhound) | Next | [ ] | Distance, bearing and ETA from self to a target |
| Elevation profile, viewshed, contour lines | No | | |
| Digital pointer | Later | [ ] | |

## 7. Communication

| ATAK-CIV feature | sitaw scope | Status | Notes |
|---|---|---|---|
| Contacts list | MVP | [~] | Team list with last-seen time, tap to pan |
| GeoChat: team ("All Chat Rooms") | Next | [ ] | Same WebSocket channel |
| GeoChat: direct messages, groups | Later | [ ] | |
| Pre-defined quick messages | Later | [ ] | |
| Emergency beacon (Alert / In Contact / Ring the Bell) | Next | [ ] | Loud alert shown to everyone |
| Video player, KLV | No | | |
| Radio controls, rover | No | | |
| VoIP / SMS / email | No | | |

## 8. Data management

| ATAK-CIV feature | sitaw scope | Status | Notes |
|---|---|---|---|
| Overlay Manager (toggle categories) | MVP | [x] | Per-folder show/hide in View; by default only your own folder is shown |
| Folders / packs (flat) | MVP | [x] | Personal folder per user; shared folders anyone can create, rename or delete when empty; move objects between folders |
| Object search | MVP | [x] | Go to: one box for people, objects, folders and coordinates; flies to the result |
| Multi-select export and delete | Later | [ ] | |
| Data packages (send bundles) | No | | Everything is shared already |
| Import Manager (KML, KMZ, GPX, GeoJSON) | Next | [ ] | GeoJSON first |
| Export (KML, GPX, GeoJSON) | Next | [ ] | |
| Hashtags and sticky tags | Later | [ ] | |
| Clear Content (wipe local data) | Later | [~] | No user-facing button; local data is wiped automatically when a removed user's session is rejected, or when joining another team |
| Encryption at rest | No | | Use HTTPS; the browser storage is the device's concern |

## 9. Network and server (what replaces the TAK Server)

| Capability | sitaw scope | Status | Notes |
|---|---|---|---|
| Single Go binary, embedded web UI | MVP | [x] | No external database |
| WebSocket sync with full snapshot on connect | MVP | [x] | |
| Last-write-wins merge with tombstones | MVP | [x] | Offline edits converge |
| Persistence | MVP | [x] | SQLite (`data/sitaw.db`); old JSON state is imported once |
| Invitation-link auth | MVP | [x] | One shared link per team; callsigns are unique within a team (case-insensitive) |
| Object links (`/i/<id>`, `/u/<id>`) | MVP | [x] | Selecting an object changes the URL; links open only for team members |
| Admin: teams, invites, members | MVP | [~] | Admin token and HTTP API; no UI yet |
| Kick a user (revoke a session) | MVP | [x] | Admin API; closes their connection and removes their marker for everyone |
| CoT (Cursor-on-Target) bridge to real ATAK | Later | [ ] | Interop with ATAK devices |
| TLS | MVP | [~] | Expected behind a reverse proxy (Caddy) |

## 10. Offline behavior

| Capability | sitaw scope | Status | Notes |
|---|---|---|---|
| App shell works offline (PWA, service worker) | MVP | [x] | |
| Last known team state shown offline | MVP | [x] | Cached in IndexedDB |
| Create, edit and delete items offline | MVP | [x] | IndexedDB outbox, flushed on reconnect |
| Own position while offline | MVP | [x] | Shown locally; only the latest fix is sent on reconnect |
| Tile cache for viewed areas | MVP | [x] | Cache-first, capped size |
| Pre-download an area's tiles | Next | [ ] | |
| Connection state indicator | MVP | [x] | |

## 11. Mobile UX

| Capability | sitaw scope | Status |
|---|---|---|
| Large touch targets (at least 56 px buttons) | MVP | [x] |
| Bottom toolbar reachable with a thumb | MVP | [x] |
| Installable PWA (manifest) | MVP | [x] |
| Keep screen awake (Wake Lock API) | Next | [ ] |
| Dark / night mode | Next | [ ] |
