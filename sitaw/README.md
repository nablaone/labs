# sitaw

A lightweight, web-based, multi-team situational awareness app inspired by ATAK-CIV.
It shares live positions, waypoints, lines and areas over an MGRS-gridded
OpenTopoMap. It is a single Go binary with an installable, offline-capable PWA.

```sh
go run ./cmd/sitaw
# open the invite URL from the log, pick a callsign
```

One server hosts many teams. Each team has its own invite link, and members
only ever see their own team's data. Create a team (the response contains its link):

```sh
SITAW_ADMIN_TOKEN=secret go run ./cmd/sitaw &
curl -XPOST -H 'Authorization: Bearer secret' -d '{"name":"Alpha"}' localhost:8080/api/admin/teams
curl -H 'Authorization: Bearer secret' localhost:8080/api/admin/teams            # teams, members, links
curl -XPOST -H 'Authorization: Bearer secret' localhost:8080/api/admin/teams/<team-id>/invites   # new link
curl -XDELETE -H 'Authorization: Bearer secret' localhost:8080/api/admin/invites/<token>         # revoke link
curl -XDELETE -H 'Authorization: Bearer secret' localhost:8080/api/admin/users/<user-id>         # remove member
```

Members can also start a team themselves: **INFO** → **New team**. INFO also
shows the team's invite link, your position, and a sign-in link that moves your
session to another device (it is a password: keep it private).

Each person in a team picks a unique callsign. Every object and person has a
link (`/i/<id>`, `/u/<id>`) that only opens for members of that team.

Data is stored in SQLite (`data/sitaw.db`).

**AI agents:** tap **AGENT** → **Connect an agent**. That creates a sub-user (e.g.
`PROBE-ALPHA`) and shows a three-line prompt: the agent's token and a `curl`
that fetches its instructions (`GET /agent`). Paste it into Claude Code (or a
similar agent). The agent can read the whole team's map and add objects, only
in its own folder. API: `/api/v1` (documented in those instructions).

**Phones need HTTPS.** Browsers only share GPS location with `https://` pages
(or `localhost`). Over plain `http://192.168.x.x:8080` the GPS status shows
**GPS needs HTTPS**. Tap it for details. The easiest options:

```sh
# Tailscale (phone also in your tailnet): https://<machine>.<tailnet>.ts.net
tailscale serve --bg 8080
# Tailscale Funnel: same, but reachable from the internet
tailscale funnel --bg 8080
# Cloudflare quick tunnel (public, temporary URL)
cloudflared tunnel --url http://localhost:8080
# Caddy on a server with a domain
caddy reverse-proxy --from sitaw.example.com --to :8080
```

Tell sitaw its external address, so invite links and agent prompts use it:

```sh
SITAW_BASE_URL=https://<machine>.<tailnet>.ts.net go run ./cmd/sitaw
```

| Env | Flag | Default | |
|---|---|---|---|
| `SITAW_BASE_URL` | `-base-url` | (derived per request) | external address clients use; set it behind a proxy or tunnel |
| `SITAW_ADDR` | `-addr` | `:8080` | listen address |
| `SITAW_DB` | `-db` | `data/sitaw.db` | SQLite file |
| `SITAW_ADMIN_TOKEN` | | random per run | bearer token for `/api/admin` |
| `SITAW_IMPORT_JSON` | `-import-json` | `data/sitaw.json` | old state file, imported once |

See `docs/FEATURES.md` for scope and `CLAUDE.md` for architecture.
