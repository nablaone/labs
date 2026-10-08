# sitaw: roadmap and feature list

sitaw is for **a small group that wants to navigate and stay aware of each
other**: friends on a hike, a search party, an event crew. It borrows from
ATAK-CIV, but leaves out what is military (symbology, fire support) or heavy
(terrain analysis, video, radios).

## Roadmap

The app already shares where everyone is and what has been marked. The gap is
helping people **get somewhere**, **talk**, and **notice when something
changes**. Items are listed in order of priority; size is S (a day or so) or M (a few days).

### Next: navigate and stay together

| # | Feature | Why | Size | Status |
|---|---|---|---|---|
| 1 | **Navigate to** (object, person, MGRS) | A live line from you to the target with distance, bearing, ETA and a turn arrow; works offline ("Bloodhound" in ATAK) | M | [ ] |
| 2 | **Team chat** | "Meet at RV-1", "running late". One channel per team over the existing socket, queued offline, unread badge | M | [ ] |
| 3 | **Breadcrumbs / tracks** | A trail behind each marker, and "where was BRAVO in the last hour". Needs a retention decision (24 h? 7 days?) | M | [ ] |
| 4 | **Range & bearing tool** | Tap two points (or person to object) for distance and bearing; reuses the line labels | S | [ ] |
| 5 | **Download map area** | Pre-fetch tiles (zoom 10–16) for an area before going out of signal | S | [ ] |
| 6 | **Screen awake + heading** | Wake Lock while navigating; your marker shows which way you face | S | [ ] |

**First slice:** 1 + 6 + 7. That makes the app useful on a walk: see everyone,
get taken to them, and shout if something is wrong. Then 2 and 3.

### Then: awareness that comes to you

| # | Feature | Why | Size | Status |
|---|---|---|---|---|
| 7 | **Help request** | "I need help / come to me": a loud banner for everyone, with navigate-to-me (the civilian emergency beacon) | S | [ ] |
| 8 | **Proximity alerts** | Notify when someone enters an area or comes within N m; or "warn if anyone is more than 1 km from the group" | M | [ ] |
| 9 | **Routes** | Follow a line: next point, distance remaining, off-route warning (builds on 1) | M | [ ] |
| 10 | **GPX / KML import and export** | Bring in a hiking route; export what was marked | S | [ ] |
| 11 | **Photos on objects** | "The gate is locked here"; needs file storage | M | [ ] |

### Later, maybe

- **Shapes:** circles and range rings ("within 500 m of camp"), editing vertices of lines and areas.
- **Elevation profile of a line:** needs an elevation data source.
- **Lat/lon display option:** for people who don't read MGRS (input already accepts degrees).
- **Push notifications while the app is closed:** web push is weak on iOS.
- **Admin page:** an in-app page for teams, invites and members, instead of `curl`.
- **MCP server for agents:** the same tools as the REST API.

### Not doing

Too military for this app, or too heavy:

- **Symbology:** affiliations (friendly/hostile/neutral), MIL-STD-2525 icons, tactical rectangle overlays.
- **Fire support:** bullseye, 9-line / CASEVAC / call-for-fire forms, digital pointer, telestrate.
- **Terrain:** viewshed, contour lines, 3D / DTED.
- **Media and radios:** video feeds / KLV, radio and rover control, VoIP/SMS integration.
- **ATAK plumbing:** data packages; a CoT bridge to real ATAK devices (only if someone in the group runs ATAK).

## Feature status

Mapped from the ATAK Civilian 4.0 user manual, plus what sitaw adds.
Status: `[x]` done, `[~]` partial, `[ ]` not started. Scope: **Done** (shipped),
**Roadmap #n**, **Later**, **No** (not doing).

### Map and coordinates

| Feature | Scope | Status | Notes |
|---|---|---|---|
| Moving map, pan / pinch zoom | Done | [x] | Leaflet |
| Base maps: OpenTopoMap, OSM | Done | [x] | Switch in View |
| MGRS coordinates and grid overlay | Done | [x] | Grid from 100 km down to 100 m, by zoom |
| Coordinate input: MGRS, lat/lon (DD, DM, DMS) | Done | [x] | Go to search |
| Lat/lon display | Later | [ ] | Display is MGRS only |
| Center on self, follow mode | Done | [x] | |
| Night mode | Done | [x] | Red on black; map reduced to dim red |
| Offline tiles for viewed areas | Done | [x] | Service worker, capped cache |
| Download map area | Roadmap #5 | [ ] | |
| Bookmarks / favorites | Later | [ ] | Waypoints cover most of it |
| Map rotation (track-up) | Later | [ ] | Leaflet has no native rotation |
| 3D, DTED, rubber sheet | No | | |

### Team awareness

| Feature | Scope | Status | Notes |
|---|---|---|---|
| Own position from GPS, shared with the team | Done | [x] | A Position object in your folder |
| Manual position / location unknown | Done | [x] | Set at the crosshair (sticks until USE GPS), or remove |
| Team marks with callsign | Done | [x] | Live / stale (>5 min) / lost (>30 min) / manual |
| GPS accuracy circle, GPS diagnostics | Done | [x] | Tap the status: needs HTTPS, denied, off, timeout |
| Team list and search | Done | [x] | Go to: roster, distance, last seen |
| Breadcrumbs / track history | Roadmap #3 | [ ] | No history is stored today |
| Heading on your marker | Roadmap #6 | [ ] | |
| Help request (emergency beacon) | Roadmap #7 | [ ] | |
| Proximity alerts / geofence | Roadmap #8 | [ ] | |
| Roles (lead, medic, ...) | Later | [ ] | |
| Affiliations, MIL-STD-2525 | No | | |

### Marking the map

| Feature | Scope | Status | Notes |
|---|---|---|---|
| Waypoint, line, area | Done | [x] | Waypoint: diamond on a point circle |
| Name, remarks, color, folder | Done | [x] | |
| Line length and grid bearing per segment, area size | Done | [x] | Shown when zoomed in |
| Line start dot and end arrow | Done | [x] | |
| Folders: personal and shared, show/hide | Done | [x] | One current folder for new objects |
| Object links (`/i/<id>`) | Done | [x] | Open only for team members |
| Circles, range rings | Later | [ ] | |
| Edit vertices | Later | [ ] | Name, color, folder and delete work |
| Photos / attachments | Roadmap #11 | [ ] | |
| GPX / KML import and export | Roadmap #10 | [ ] | |
| Telestrate, tactical rectangles, iconsets | No | | |

### Navigation

| Feature | Scope | Status | Notes |
|---|---|---|---|
| Go to (fly the map to a place) | Done | [x] | |
| Navigate to (Bloodhound) | Roadmap #1 | [ ] | Distance, bearing, ETA, turn arrow |
| Range & bearing tool | Roadmap #4 | [ ] | |
| Routes with checkpoints | Roadmap #9 | [ ] | |
| Elevation profile | Later | [ ] | |
| Bullseye, digital pointer, viewshed | No | | |

### Communication

| Feature | Scope | Status | Notes |
|---|---|---|---|
| Team chat | Roadmap #2 | [ ] | |
| Direct messages, quick messages | Later | [ ] | |
| Video, radio, VoIP, SMS | No | | |

### Server, sync and accounts

| Feature | Scope | Status | Notes |
|---|---|---|---|
| Single Go binary, embedded PWA | Done | [x] | |
| SQLite storage, automatic migrations | Done | [x] | |
| Multiple teams, isolated data | Done | [x] | |
| Invite link per team, unique callsigns | Done | [x] | |
| Sign-in link to move a session to another device | Done | [x] | INFO panel |
| WebSocket sync, last-write-wins, offline outbox | Done | [x] | |
| External address config (`SITAW_BASE_URL`) | Done | [x] | Needed behind proxies / tailscale serve |
| AI agents: sub-users, REST `/api/v1`, geo API | Done | [x] | Agents write only in their own folder |
| Admin API: teams, invites, remove members | Done | [~] | No admin UI yet (Later) |
| TLS | Done | [~] | Via a reverse proxy or tunnel |
| MCP server for agents | Later | [ ] | |
| CoT bridge to ATAK | No | | Unless the group uses ATAK |

### Mobile UX

| Feature | Scope | Status | Notes |
|---|---|---|---|
| Large labelled tiles, panels in one place | Done | [x] | Top-right column |
| Installable PWA, works offline | Done | [x] | |
| Keep screen awake | Roadmap #6 | [ ] | Wake Lock API |
| Push notifications when closed | Later | [ ] | |
