# sitaw

A lightweight, web-based, multi-team situational awareness app inspired by ATAK-CIV.
It shares live positions, waypoints, lines and areas over an MGRS-gridded
OpenTopoMap. It is a single Go binary with an installable, offline-capable PWA.

```sh
go run ./cmd/sitaw -base-url http://localhost:8080
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

Each person in a team picks a unique callsign. Every object and person has a
link (`/i/<id>`, `/u/<id>`) that only opens for members of that team.

Data is stored in SQLite (`data/sitaw.db`).

For phones, serve over HTTPS (GPS and offline mode require it), e.g. `caddy reverse-proxy --to :8080`.

See `docs/FEATURES.md` for scope and `CLAUDE.md` for architecture.
