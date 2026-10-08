// sitaw client entry point: auth, map, UI wiring.
//
// UI layout: everything floats over the map. Round buttons in the top-right
// (View, Go to, Objects, Draw) open popovers to their left; the bottom-left
// corner shows callsign, center MGRS, GPS and server status. Forms use a
// bottom sheet. Objects live in flat folders (one personal folder per user).

import { Sync } from './sync.js';
import { ItemsLayer, TeamLayer, SelfMarker, DEFAULT_COLORS, lineLength, polygonArea } from './layers.js';
import { DrawTool } from './draw.js';
import { MgrsGrid } from './mgrsgrid.js';
import { formatCoord, formatDegrees, parseAnyCoord, formatDistance } from './coords.js';
import { icon, hydrateIcons } from './icons.js';
import * as db from './db.js';

const SESSION_KEY = 'sitaw.session';
const PREFS_KEY = 'sitaw.view';
const POS_MIN_INTERVAL = 3000;   // ms between position sends while moving
const POS_HEARTBEAT = 30000;     // resend even when stationary
const POS_MIN_MOVE = 5;          // meters
const GPS_STALE = 30000;         // a fix older than this is shown as stale
const COLORS = ['#e53935', '#fb8c00', '#fdd835', '#43a047', '#00acc1', '#1e88e5', '#8e24aa', '#000000', '#ffffff'];
const KIND_LABEL = { waypoint: 'Waypoint', line: 'Line', area: 'Area' };
const KIND_PREFIX = { waypoint: 'WP', line: 'LN', area: 'AR' };
const KIND_ICON = { waypoint: 'point', line: 'line', area: 'area' };

const $ = (sel, root = document) => root.querySelector(sel);
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);

// ---------- session / join ----------

function loadSession() {
  try { return JSON.parse(localStorage.getItem(SESSION_KEY)); } catch { return null; }
}

async function leave() {
  localStorage.removeItem(SESSION_KEY);
  await db.wipe();
  location.replace('/');
}

async function showJoin(invite, session) {
  const msg = $('#join-msg');
  $('#join').hidden = false;
  let team = null;
  try {
    const r = await fetch(`/api/invites/${encodeURIComponent(invite)}`);
    if (r.ok) team = (await r.json()).team;
  } catch { /* offline: try anyway on submit */ }
  if (team && session?.team?.id === team.id) {
    location.replace('/'); // already a member on this device
    return;
  }
  if (team) {
    msg.innerHTML = `Join team <b>${esc(team.name)}</b>.`;
    if (session) msg.innerHTML += ` This device is in team <b>${esc(session.team?.name ?? '?')}</b> now; joining signs you out of it.`;
  } else if (navigator.onLine) {
    msg.textContent = 'This invite link is invalid or has expired. Ask your team lead for a new one.';
    $('#join-form').querySelector('button').disabled = true;
  }
  $('#join-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const err = $('#join-err');
    err.hidden = true;
    try {
      const r = await fetch('/api/join', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ invite, callsign: $('#callsign').value }),
      });
      const body = await r.json();
      if (!r.ok) throw new Error(body.error || r.statusText);
      await db.wipe();
      localStorage.setItem(SESSION_KEY, JSON.stringify(body));
      location.replace('/');
    } catch (ex) {
      err.textContent = navigator.onLine ? ex.message : 'You are offline. Joining needs a connection.';
      err.hidden = false;
    }
  });
}

// Per-device view preferences: base map, grid/team overlays, which folders
// are shown, and where new objects go. Folder visibility is stored only once
// the user toggles it; until then folderVisible() picks the default.
function loadPrefs() {
  const def = { base: 'topo', show: { grid: true, team: true }, folders: {}, drawFolder: null, folded: {}, night: false };
  try {
    const p = JSON.parse(localStorage.getItem(PREFS_KEY));
    if (!p) return def;
    return {
      base: p.base ?? def.base,
      show: { grid: p.show?.grid ?? true, team: p.show?.team ?? true },
      folders: p.folders ?? {},
      drawFolder: p.drawFolder ?? null,
      folded: p.folded ?? {},
      night: !!p.night,
    };
  } catch {
    return def;
  }
}

function savePrefs(p) {
  try { localStorage.setItem(PREFS_KEY, JSON.stringify(p)); } catch { /* private mode */ }
}

// ---------- main app ----------

function startApp(session) {
  $('#app').hidden = false;
  hydrateIcons($('#app'));
  const me = session.user;
  const prefs = loadPrefs();

  const map = L.map('map', { zoomControl: false, attributionControl: true }).setView([52.0, 19.0], 6);
  map.attributionControl.setPrefix(false);
  const bases = {
    topo: L.tileLayer('https://{s}.tile.opentopomap.org/{z}/{x}/{y}.png', {
      maxZoom: 17, subdomains: 'abc', crossOrigin: true,
      attribution: '&copy; OpenStreetMap, SRTM | &copy; OpenTopoMap (CC-BY-SA)',
    }),
    osm: L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19, crossOrigin: true, attribution: '&copy; OpenStreetMap',
    }),
  };

  // Status box is a Leaflet control so the scale bar stacks neatly above it.
  const StatusControl = L.Control.extend({
    onAdd: () => {
      const el = $('#status');
      L.DomEvent.disableClickPropagation(el);
      return el;
    },
  });
  new StatusControl({ position: 'bottomleft' }).addTo(map);
  L.control.scale({ imperial: false, position: 'bottomleft' }).addTo(map);
  const renderMe = () => {
    $('#me-line').innerHTML = `${esc(me.callsign)} <span class="team">· ${esc(session.team?.name ?? '')}</span>`;
  };
  renderMe();

  const sync = new Sync(session.token, me.id, session.team?.id);

  // ----- folders -----
  // A folder is an item of kind "folder"; map objects point at one with
  // `folder`. Every user has a personal folder (roster: user.folderId).
  const isFolder = (it) => it?.kind === 'folder';
  const live = (it) => it && !it.deleted;
  const myFolder = () => me.folderId ?? sync.users.get(me.id)?.folderId;
  const personalOwner = (fid) => [...sync.users.values()].find((u) => u.folderId === fid);
  const allFolders = () => {
    const mine = myFolder();
    const rank = (f) => (f.id === mine ? 0 : personalOwner(f.id) ? 2 : 1); // mine, shared, other people's
    return sync.liveItems().filter(isFolder)
      .sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name));
  };
  // The folder an object is shown in. Objects whose folder vanished (deleted
  // while someone was offline) fall back to their creator's personal folder.
  const folderOf = (it) => {
    const f = sync.items.get(it.folder);
    if (live(f) && isFolder(f)) return it.folder;
    return sync.users.get(it.createdBy)?.folderId ?? it.folder ?? '';
  };
  const folderName = (fid) => sync.items.get(fid)?.name ?? 'Unknown folder';
  // By default you see your own objects only: your folder and folders you made.
  const folderVisible = (fid) => prefs.folders[fid] ?? (fid === myFolder() || sync.items.get(fid)?.createdBy === me.id);
  const setFolderVisible = (fid, on) => {
    prefs.folders[fid] = on;
    savePrefs(prefs);
    items.reset(sync.liveItems());
  };
  const drawTarget = () => {
    const f = sync.items.get(prefs.drawFolder);
    return live(f) && isFolder(f) ? f.id : myFolder();
  };
  const objectsIn = (fid) => sync.liveItems().filter((i) => !isFolder(i) && folderOf(i) === fid);

  const items = new ItemsLayer(map, (id, latlng) => onItemTap(id, latlng), (it) => folderVisible(folderOf(it)));

  // Selected object: a yellow outline drawn just below the objects.
  map.createPane('selection').style.zIndex = 390;
  const selection = L.layerGroup().addTo(map);
  let selectedId = null;
  function select(it) {
    selection.clearLayers();
    selectedId = it && !isFolder(it) ? it.id : null;
    if (!selectedId) return;
    const style = { pane: 'selection', color: '#ffeb3b', opacity: 0.9, interactive: false };
    if (it.kind === 'waypoint') L.circleMarker(it.coords[0], { ...style, radius: 24, weight: 5, fill: false }).addTo(selection);
    else if (it.kind === 'line') L.polyline(it.coords, { ...style, weight: 14 }).addTo(selection);
    else L.polygon(it.coords, { ...style, weight: 12, fill: false }).addTo(selection);
  }
  const team = new TeamLayer(map, me.id, (uid) => { closePops(); openUser(uid, false); });
  const self = new SelfMarker(map);
  const grid = new MgrsGrid();

  const overlays = { grid, team: team.group };
  function applyPrefs() {
    for (const [k, l] of Object.entries(bases)) {
      if (k === prefs.base) l.addTo(map); else l.remove();
    }
    for (const [k, l] of Object.entries(overlays)) {
      if (prefs.show[k]) l.addTo(map); else l.remove();
    }
    // Night: red-on-black UI and a dimmed red map, to keep dark adaptation.
    document.documentElement.toggleAttribute('data-night', prefs.night);
    $('meta[name="theme-color"]').content = prefs.night ? '#000000' : '#ffffff';
    savePrefs(prefs);
  }
  applyPrefs();

  // Restore last view.
  db.get('kv', 'view').then((v) => v && map.setView([v.lat, v.lon], v.zoom));
  map.on('moveend', () => {
    const c = map.getCenter();
    db.put('kv', 'view', { lat: c.lat, lon: c.lng, zoom: map.getZoom() });
  });

  // Center coordinate readout.
  const updateCenter = () => {
    const c = map.getCenter();
    $('#center-coord').textContent = formatCoord(c.lat, c.lng);
  };
  map.on('move', updateCenter);
  updateCenter();

  // ----- status: server -----
  function setStatus(el, cls, ic, text) {
    el.className = `st ${cls}`;
    el.innerHTML = `${icon(ic)}<span class="t">${esc(text)}</span>`;
  }
  const updateServer = () => {
    const n = sync.pendingCount;
    const q = n ? ` · ${n} queued` : '';
    if (sync.online) setStatus($('#srv-st'), n ? 'warn' : 'ok', 'cloud', `live${q}`);
    else setStatus($('#srv-st'), 'bad', 'cloudOff', `offline${q}`);
  };
  // Re-render an open popover when the data behind it changes.
  const refreshPop = () => {
    if (openPop === 'objects') renderObjects($('[data-pop="objects"]'));
    if (openPop === 'view') renderView($('[data-pop="view"]'));
  };
  sync.addEventListener('status', updateServer);
  sync.addEventListener('reset', (e) => {
    items.reset(e.detail.items);
    updateServer();
    refreshPop();
    const final = e.detail.source === 'server';
    resolveRoute(final);
    // The server copy is authoritative: close an open object it doesn't have.
    const r = parseRoute();
    if (final && r?.kind === 'item' && !live(sync.items.get(r.id))) {
      hideSheet();
      history.replaceState(null, '', '/');
      toast('That object is not part of your team (or was deleted)');
    }
  });
  sync.addEventListener('you', (e) => {
    let changed = false;
    if (e.detail.team && e.detail.team.id !== session.team?.id) {
      session.team = e.detail.team; // sessions from before teams existed
      changed = true;
    }
    if (e.detail.user?.folderId && me.folderId !== e.detail.user.folderId) {
      me.folderId = e.detail.user.folderId; // sessions from before folders existed
      changed = true;
    }
    if (changed) {
      localStorage.setItem(SESSION_KEY, JSON.stringify(session));
      renderMe();
    }
  });
  sync.addEventListener('item', (e) => {
    const it = e.detail.item;
    if (it.id === selectedId) select(live(it) ? it : null);
    // A folder change can move many objects in or out of view.
    if (isFolder(it)) items.reset(sync.liveItems()); else items.upsert(it);
    updateServer();
    refreshPop();
  });
  sync.addEventListener('user', refreshPop);
  sync.addEventListener('positions', (e) => team.reset(e.detail.positions));
  sync.addEventListener('pos', (e) => team.set(e.detail.pos));
  sync.addEventListener('leave', (e) => team.remove(e.detail.userId));
  sync.addEventListener('rejected', (e) => toast(`Not saved: ${e.detail.error}`));
  sync.addEventListener('unauthorized', () => {
    toast('Your session is no longer valid.');
    setTimeout(leave, 2000);
  });
  sync.start();
  setInterval(updateServer, 2000);

  // ----- own position + status: GPS -----
  let follow = false, lastFix = null, lastSent = null, gpsError = null;
  const updateGps = () => {
    const el = $('#gps-st');
    if (gpsError) setStatus(el, 'bad', 'gps', gpsError);
    else if (!lastFix) setStatus(el, 'warn', 'gps', 'GPS…');
    else if (Date.now() - lastFix.ts > GPS_STALE) setStatus(el, 'warn', 'gps', `GPS ${ago(lastFix.ts)}`);
    else setStatus(el, 'ok', 'gps', `±${Math.round(lastFix.acc)} m${follow ? ' · follow' : ''}`);
  };
  const setFollow = (on) => {
    follow = on;
    $('[data-fab="goto"]').classList.toggle('following', on);
    updateGps();
  };
  const sendFix = (force) => {
    if (!lastFix) return;
    const now = Date.now();
    const moved = lastSent ? L.latLng(lastSent.lat, lastSent.lon).distanceTo([lastFix.lat, lastFix.lon]) : Infinity;
    const since = lastSent ? now - lastSent.at : Infinity;
    if (force || since >= POS_HEARTBEAT || (moved >= POS_MIN_MOVE && since >= POS_MIN_INTERVAL)) {
      sync.sendPos(lastFix);
      lastSent = { lat: lastFix.lat, lon: lastFix.lon, at: now };
    }
  };
  if ('geolocation' in navigator) {
    navigator.geolocation.watchPosition((p) => {
      const c = p.coords;
      gpsError = null;
      lastFix = {
        lat: c.latitude, lon: c.longitude, acc: c.accuracy, ts: p.timestamp,
        ...(c.heading != null && !Number.isNaN(c.heading) ? { hdg: c.heading } : {}),
        ...(c.speed != null ? { spd: c.speed } : {}),
      };
      self.update(c.latitude, c.longitude, c.accuracy);
      if (follow) map.panTo([c.latitude, c.longitude]);
      sendFix(false);
      updateGps();
    }, (err) => {
      gpsError = err.code === err.PERMISSION_DENIED ? 'GPS denied' : 'no GPS';
      updateGps();
    }, { enableHighAccuracy: true, maximumAge: 5000 });
    setInterval(() => { sendFix(false); updateGps(); }, 5000);
  } else {
    gpsError = 'no GPS';
  }
  updateGps();
  map.on('dragstart', () => { if (follow) setFollow(false); });

  function goToMe() {
    if (!lastFix) { toast(gpsError ?? 'Waiting for GPS…'); return; }
    setFollow(true);
    map.setView([lastFix.lat, lastFix.lon], Math.max(map.getZoom(), 15));
  }

  // ----- popovers (View / Go to / Objects / Draw) -----
  let openPop = null;
  function closePops() {
    for (const p of document.querySelectorAll('.pop')) p.hidden = true;
    for (const b of document.querySelectorAll('[data-fab]')) b.classList.toggle('active', b.dataset.fab === 'draw' && draw.active);
    openPop = null;
  }
  function togglePop(name) {
    const was = openPop;
    closePops();
    closeSheet();
    if (was === name) return;
    if (name === 'draw' && draw.active) { draw.cancel(); return; } // tap Draw again to abort
    const pop = $(`[data-pop="${name}"]`);
    openPop = name;
    if (name === 'view') renderView(pop);
    if (name === 'goto') renderGoto(pop);
    if (name === 'objects') renderObjects(pop);
    pop.hidden = false;
    $(`[data-fab="${name}"]`).classList.add('active');
  }
  $('#fab').addEventListener('click', (e) => {
    const fab = e.target.closest('[data-fab]')?.dataset.fab;
    if (fab) togglePop(fab);
  });
  L.DomEvent.disableClickPropagation($('#fab'));
  L.DomEvent.disableScrollPropagation($('#fab'));
  map.on('click', () => {
    if (draw.active) return;
    closePops();
    closeSheet();
  });

  const chip = (attr, key, ic, label, on, extra = '') =>
    `<button class="chip${on ? ' on' : ''}" ${attr}="${esc(key)}">${icon(ic)}${esc(label)}${extra}</button>`;

  function renderView(pop) {
    const folders = allFolders();
    pop.innerHTML = `
      <h4>Base map</h4>
      <div class="chips">
        ${chip('data-base', 'topo', 'topo', 'Topo', prefs.base === 'topo')}
        ${chip('data-base', 'osm', 'map', 'OSM', prefs.base === 'osm')}
      </div>
      <h4>Show</h4>
      <div class="chips">
        ${chip('data-show', 'grid', 'grid', 'Grid', prefs.show.grid)}
        ${chip('data-show', 'team', 'user', 'Team', prefs.show.team)}
        ${chip('data-night', 'night', 'moon', 'Night', prefs.night)}
      </div>
      <h4>Folders</h4>
      <div class="pop-actions">
        <button class="chip" data-vis="mine">${icon('user')}Only mine</button>
        <button class="chip" data-vis="all">${icon('eye')}All</button>
      </div>
      <div class="chips">
        ${folders.map((f) => chip('data-folder', f.id, folderVisible(f.id) ? 'eye' : 'eyeOff', f.name, folderVisible(f.id),
          ` <span class="count">${objectsIn(f.id).length}</span>`)).join('') || '<div class="empty">No folders yet</div>'}
      </div>
      <h4>${esc(me.callsign)} · ${esc(session.team?.name ?? 'team')}</h4>
      <button class="prow" data-act="leave">${icon('logout')}<span class="name">Leave team…</span></button>`;
    pop.onclick = (e) => {
      const t = e.target;
      const base = t.closest('[data-base]')?.dataset.base;
      const show = t.closest('[data-show]')?.dataset.show;
      const fid = t.closest('[data-folder]')?.dataset.folder;
      const vis = t.closest('[data-vis]')?.dataset.vis;
      if (base) { prefs.base = base; applyPrefs(); }
      else if (show) { prefs.show[show] = !prefs.show[show]; applyPrefs(); }
      else if (t.closest('[data-night]')) { prefs.night = !prefs.night; applyPrefs(); }
      else if (fid) setFolderVisible(fid, !folderVisible(fid));
      else if (vis) {
        for (const f of folders) prefs.folders[f.id] = vis === 'all' || f.id === myFolder();
        setFolderVisible(myFolder(), true);
      } else if (t.closest('[data-act="leave"]')) { closePops(); confirmLeave(); return; }
      else return;
      renderView(pop);
    };
  }

  // Go to: one search box. It takes a position (MGRS or lat/lon degrees) and
  // searches people, folders and objects. Picking a result only moves the map
  // (no details; those live in the Objects menu). Enter picks the first result.
  function renderGoto(pop) {
    pop.innerHTML = `
      <div class="inrow search">
        ${icon('search')}
        <input name="q" type="search" enterkeyhint="go" autocomplete="off" autocapitalize="off" spellcheck="false"
          placeholder="Search, MGRS or lat, lon">
      </div>
      <div class="results"></div>`;
    const results = $('.results', pop);
    const input = $('input[name="q"]', pop);
    const renderResults = () => { results.innerHTML = searchResults(input.value); };
    input.oninput = renderResults;
    input.onkeydown = (e) => {
      if (e.key !== 'Enter') return;
      e.preventDefault();
      const first = results.querySelector('[data-go], [data-user], [data-item]');
      if (first) pick(first);
      else toast('Nothing found');
    };
    renderResults();
    setTimeout(() => input.focus(), 0);
    pop.onclick = (e) => {
      const row = e.target.closest('[data-go], [data-user], [data-item]');
      if (row) pick(row);
    };
  }

  function pick(row) {
    const { go, user, item } = row.dataset;
    closePops();
    if (go === 'me') { goToMe(); return; }
    setFollow(false);
    if (go === 'coord') {
      const lat = Number(row.dataset.lat), lon = Number(row.dataset.lon), prec = Number(row.dataset.prec);
      // Coarser references get a wider view.
      const zoom = prec >= 100000 ? 9 : prec >= 10000 ? 11 : prec >= 1000 ? 14 : 16;
      map.setView([lat, lon], zoom);
      return;
    }
    if (user) {
      const p = sync.positions.get(user);
      if (p) map.setView([p.lat, p.lon], Math.max(map.getZoom(), 14));
      else toast('No position shared yet');
      return;
    }
    const it = sync.items.get(item);
    if (!live(it)) return;
    const fid = isFolder(it) ? it.id : folderOf(it);
    if (!folderVisible(fid)) { setFolderVisible(fid, true); toast(`Showing folder ${folderName(fid)}`); }
    if (isFolder(it)) fitFolder(it.id); else zoomTo(it);
  }

  function searchResults(raw) {
    const q = raw.trim().toLowerCase();
    const here = lastFix ? L.latLng(lastFix.lat, lastFix.lon) : map.getCenter();
    const dist = (ll) => formatDistance(here.distanceTo(ll));
    const people = [...sync.positions.values()].filter((p) => p.userId !== me.id);
    const meRow = `<button class="prow" data-go="me">${icon('me')}<span class="name">Me${follow ? ' (following)' : ''}</span>
      <span class="meta">${lastFix ? `±${Math.round(lastFix.acc)} m` : esc(gpsError ?? 'no fix')}</span></button>`;
    if (!q) {
      return `${meRow}<h4>Team · ${people.length}</h4>${people.sort((a, b) => b.ts - a.ts).map((p) => personRow(p, dist)).join('')
        || '<div class="empty">No one else yet</div>'}`;
    }
    const c = parseAnyCoord(raw);
    const coordRow = c && `<h4>Position</h4><button class="prow" data-go="coord" data-lat="${c.lat}" data-lon="${c.lon}" data-prec="${c.precision}">
      ${icon('go')}<span class="name">${esc(formatCoord(c.lat, c.lon))}</span>
      <span class="meta">${esc(c.kind === 'mgrs' ? formatDegrees(c.lat, c.lon) : dist([c.lat, c.lon]))}</span></button>`;
    const match = (...s) => s.some((x) => x?.toLowerCase().includes(q));
    const ppl = people.filter((p) => match(p.callsign));
    const folders = allFolders().filter((f) => match(f.name));
    const objs = sync.liveItems().filter((i) => !isFolder(i) && match(i.name, i.remarks, folderName(folderOf(i))))
      .sort((a, b) => a.name.localeCompare(b.name)).slice(0, 50);
    const out = [
      coordRow,
      ppl.length && `<h4>Team</h4>${ppl.map((p) => personRow(p, dist)).join('')}`,
      objs.length && `<h4>Objects</h4>${objs.map((i) => objectRow(i, dist(anchor(i)), true)).join('')}`,
      folders.length && `<h4>Folders</h4>${folders.map((f) => folderButton(f)).join('')}`,
    ].filter(Boolean).join('');
    return out || '<div class="empty">Nothing found. Try a name, MGRS (34UDC1234) or lat, lon (52.23, 21.01).</div>';
  }

  const anchor = (it) => it.kind === 'waypoint' ? L.latLng(it.coords[0]) : L.latLngBounds(it.coords).getCenter();
  const personRow = (p, dist) => `<button class="prow" data-user="${esc(p.userId)}">${icon('user')}<span class="name">${esc(p.callsign)}</span>
    <span class="meta">${dist([p.lat, p.lon])} · ${ago(p.ts)}</span></button>`;
  const folderButton = (f) => `<button class="prow" data-item="${esc(f.id)}">${icon('folder')}<span class="name">${esc(f.name)}</span>
    <span class="meta">${objectsIn(f.id).length}</span></button>`;
  // Only the icon carries the object's color; text stays ink for legibility.
  const objectRow = (i, meta, showFolder = false) => `<button class="prow${folderVisible(folderOf(i)) ? '' : ' dim'}" data-item="${esc(i.id)}">
    <span class="swatch-icon" style="color:${esc(i.color || DEFAULT_COLORS[i.kind])}">${icon(KIND_ICON[i.kind])}</span>
    <span class="name">${esc(i.name || KIND_LABEL[i.kind])}</span>
    <span class="meta">${showFolder ? `${esc(folderName(folderOf(i)))} · ` : ''}${esc(meta)}</span></button>`;

  // Objects: every folder with its objects. Picking an object opens its details.
  // Folding a folder hides its contents in this list (not on the map; that's
  // View). Remembered per device; by default only your own folder is unfolded.
  const isFolded = (fid) => prefs.folded[fid] ?? fid !== myFolder();
  const setFolded = (fid, on) => { prefs.folded[fid] = on; savePrefs(prefs); };
  function renderObjects(pop) {
    const target = drawTarget();
    const folders = allFolders();
    const anyOpen = folders.some((f) => !isFolded(f.id));
    const scroll = pop.scrollTop; // keep the place while browsing when data changes
    pop.innerHTML = `
      <div class="pop-actions">
        <button class="chip" data-act="new-folder">${icon('folderPlus')}New folder</button>
        ${folders.length > 1 ? `<button class="chip" data-act="${anyOpen ? 'fold-all' : 'unfold-all'}">${icon('chevron')}${anyOpen ? 'Fold all' : 'Unfold all'}</button>` : ''}
      </div>
      ${folders.map((f) => {
        const objs = objectsIn(f.id).sort((a, b) => b.updatedAt - a.updatedAt);
        const open = !isFolded(f.id);
        const owner = personalOwner(f.id);
        return `<div class="frow">
            <button class="prow${folderVisible(f.id) ? '' : ' dim'}" data-toggle="${esc(f.id)}" aria-expanded="${open}">
              <span class="chev${open ? ' open' : ''}">${icon('chevron')}</span>${icon(folderVisible(f.id) ? 'folder' : 'eyeOff')}
              <span class="name">${esc(f.name)}</span>
              ${owner ? `<span class="tag" title="Personal folder">${icon('user')}</span>` : ''}
              ${f.id === target ? `<span class="tag" title="New objects go here">${icon('star')}</span>` : ''}
              <span class="meta">${objs.length}</span>
            </button>
            <button class="mini" data-folder-menu="${esc(f.id)}" aria-label="Folder options">${icon('more')}</button>
          </div>
          ${open ? `<div class="sublist">${objs.map((i) => `<div class="frow">${objectRow(i, KIND_LABEL[i.kind])}
            <button class="mini" data-details="${esc(i.id)}" aria-label="Details">${icon('info')}</button></div>`).join('')
            || '<div class="empty">Empty</div>'}</div>` : ''}`;
      }).join('') || '<div class="empty">No folders yet</div>'}`;
    pop.scrollTop = scroll;
    pop.onclick = (e) => {
      const t = e.target;
      const tog = t.closest('[data-toggle]')?.dataset.toggle;
      const menu = t.closest('[data-folder-menu]')?.dataset.folderMenu;
      const i = t.closest('[data-item]')?.dataset.item;
      const details = t.closest('[data-details]')?.dataset.details;
      if (tog) {
        setFolded(tog, !isFolded(tog));
        renderObjects(pop);
      } else if (menu) { closePops(); openItem(menu, false); }
      else if (t.closest('[data-act="new-folder"]')) { closePops(); newFolder(); }
      else if (t.closest('[data-act="fold-all"], [data-act="unfold-all"]')) {
        const fold = !!t.closest('[data-act="fold-all"]');
        for (const f of folders) prefs.folded[f.id] = fold;
        savePrefs(prefs);
        renderObjects(pop);
      }
      else if (details) { closePops(); openItem(details, false); }
      else if (i) {
        // Fly to it and keep the menu open, so you can step through objects.
        setFollow(false);
        const it = sync.items.get(i);
        const fid = folderOf(it);
        if (!folderVisible(fid)) { setFolderVisible(fid, true); toast(`Showing folder ${folderName(fid)}`); }
        hideSheet();
        navigate(`/i/${i}`);
        select(it);
        focusItem(it, pop);
        markSelected(pop);
      }
    };
    markSelected(pop);
  }

  function markSelected(pop) {
    for (const r of pop.querySelectorAll('[data-item]')) r.classList.toggle('sel', r.dataset.item === selectedId);
  }

  // Frames an object in the part of the map not covered by the open menu.
  function focusItem(it, pop) {
    const h = map.getSize().y;
    const top = Math.min(pop && !pop.hidden ? pop.getBoundingClientRect().bottom + 16 : 60, h * 0.75);
    const pts = it.kind === 'waypoint' ? [it.coords[0], it.coords[0]] : it.coords;
    map.fitBounds(L.latLngBounds(pts), {
      paddingTopLeft: [24, top], paddingBottomRight: [80, 90],
      maxZoom: it.kind === 'waypoint' ? Math.max(map.getZoom(), 15) : 17,
    });
  }

  // ----- drawing -----
  const draw = new DrawTool(map, {
    onChange: (kind, n, min) => {
      $('#drawctl').hidden = !kind;
      $('[data-fab="draw"]').classList.toggle('active', !!kind);
      $('[data-fab="draw"]').innerHTML = `${icon(kind ? 'x' : 'pencil')}<span class="lbl">${kind ? 'Cancel' : 'Draw'}</span>`;
      if (!kind) return;
      const into = ` → ${folderName(drawTarget())}`;
      $('#draw-hint').textContent = kind === 'waypoint'
        ? `Tap map or +${into}`
        : `${KIND_LABEL[kind]} · ${n} pt${n === 1 ? '' : 's'}${n < min ? ` (min ${min})` : ''}${into}`;
      $('[data-draw="done"]').hidden = kind === 'waypoint';
      $('[data-draw="undo"]').hidden = kind === 'waypoint' || n === 0;
    },
    onFinish: async (kind, coords) => {
      const folder = drawTarget();
      const n = sync.liveItems().filter((i) => i.kind === kind).length + 1;
      if (folder && !folderVisible(folder)) setFolderVisible(folder, true);
      const it = await sync.putItem({
        id: newId(), kind, coords, folder, name: `${KIND_PREFIX[kind]}-${n}`, color: DEFAULT_COLORS[kind], remarks: '',
      });
      navigate(`/i/${it.id}`);
      editItem(it.id);
    },
  });
  $('[data-pop="draw"]').addEventListener('click', (e) => {
    const kind = e.target.closest('[data-draw-kind]')?.dataset.drawKind;
    if (!kind) return;
    closePops();
    draw.start(kind);
  });
  L.DomEvent.disableClickPropagation($('#drawctl'));
  $('#drawctl').addEventListener('click', (e) => {
    const a = e.target.closest('[data-draw]')?.dataset.draw;
    if (a === 'center') draw.addCenter();
    else if (a === 'undo') draw.undo();
    else if (a === 'cancel') draw.cancel();
    else if (a === 'done' && !draw.finish()) toast('Not enough points yet');
  });

  function onItemTap(id, latlng) {
    const it = sync.items.get(id);
    if (draw.active) {
      // Tapping a waypoint snaps the vertex onto it; on a shape, use the tap point.
      draw.add(it?.kind === 'waypoint' ? L.latLng(it.coords[0]) : latlng);
      return;
    }
    closePops();
    openItem(id, false);
  }

  // ----- links: every object has a URL (/i/<item id>, /u/<user id>) -----
  // The URL only holds the id; the object itself comes from this team's
  // replica, so a link is useless to anyone outside the team.
  const parseRoute = () => {
    const m = /^\/([iu])\/([0-9a-f-]{8,36})$/.exec(location.pathname);
    return m ? { kind: m[1] === 'i' ? 'item' : 'user', id: m[2] } : null;
  };
  const navigate = (path) => {
    if (location.pathname !== path) history.pushState(null, '', path);
  };
  let pendingRoute = parseRoute();

  // Opens the object named by the URL once it is known locally. final=true
  // means the server snapshot is in, so a miss really is "not found".
  function resolveRoute(final) {
    const r = pendingRoute;
    if (!r) return;
    const found = r.kind === 'item' ? live(sync.items.get(r.id)) : sync.users.has(r.id) || sync.positions.has(r.id);
    if (found) {
      pendingRoute = null;
      if (r.kind === 'item') {
        const it = sync.items.get(r.id);
        const fid = isFolder(it) ? it.id : folderOf(it);
        if (!folderVisible(fid)) setFolderVisible(fid, true); // a shared link should show its object
        openItem(r.id, true);
      } else openUser(r.id, true);
    } else if (final) {
      pendingRoute = null;
      history.replaceState(null, '', '/');
      toast('That link is not part of your team (or was deleted)');
    }
  }
  window.addEventListener('popstate', () => {
    pendingRoute = parseRoute();
    if (!pendingRoute) { hideSheet(); select(null); }
    resolveRoute(true);
  });

  function openItem(id, zoom) {
    const it = sync.items.get(id);
    if (!live(it)) return;
    navigate(`/i/${id}`);
    if (isFolder(it)) {
      if (zoom) fitFolder(id);
      showFolder(id);
      return;
    }
    if (zoom) zoomTo(it);
    select(it);
    showItem(id);
  }

  function openUser(uid, zoom) {
    navigate(`/u/${uid}`);
    const p = sync.positions.get(uid);
    if (zoom && p) {
      setFollow(false);
      map.setView([p.lat, p.lon], Math.max(map.getZoom(), 14));
    }
    showUser(uid);
  }

  async function copyLink() {
    const url = location.origin + location.pathname;
    try {
      await navigator.clipboard.writeText(url);
      toast('Link copied · only your team can open it');
    } catch {
      toast(url);
    }
  }

  // ----- bottom sheet -----
  function openSheet(html) {
    const s = $('#sheet');
    $('.sheet-body', s).innerHTML = `<button class="sheet-x" data-sheet-close aria-label="Close">${icon('x')}</button>${html}`;
    s.hidden = false;
    s.scrollTop = 0;
    return s;
  }
  $('#sheet').addEventListener('click', (e) => { if (e.target.closest('[data-sheet-close]')) closeSheet(); });
  function hideSheet() {
    $('#sheet').hidden = true;
  }
  // Closing the sheet deselects: the URL goes back to the plain map.
  function closeSheet() {
    hideSheet();
    select(null);
    navigate('/');
  }

  function describe(it) {
    if (it.kind === 'waypoint') return formatCoord(...it.coords[0]);
    if (it.kind === 'line') return `${it.coords.length} pts · ${formatDistance(lineLength(it.coords))}`;
    const a = polygonArea(it.coords);
    return `${it.coords.length} pts · ${a < 1e6 ? `${Math.round(a)} m²` : `${(a / 1e6).toFixed(2)} km²`}`;
  }

  function author(uid) {
    return sync.users.get(uid)?.callsign ?? sync.positions.get(uid)?.callsign ?? (uid === me.id ? me.callsign : uid ? uid.slice(0, 6) : 'system');
  }

  function showItem(id) {
    const it = sync.items.get(id);
    if (!live(it)) return;
    const fid = folderOf(it);
    const s = openSheet(`
      <h2>${esc(it.name || KIND_LABEL[it.kind])}</h2>
      <div class="sub">${esc(KIND_LABEL[it.kind])} · ${esc(describe(it))}</div>
      <div class="sub">in <button class="folder-link" data-a="folder">${esc(folderName(fid))}</button></div>
      ${it.remarks ? `<p>${esc(it.remarks)}</p>` : ''}
      <div class="sub">by ${esc(author(it.createdBy))} · ${new Date(it.updatedAt).toLocaleString()}</div>
      <div class="row actions">
        <button class="btn" data-a="zoom">Zoom</button>
        <button class="btn" data-a="edit">Edit</button>
        <button class="btn icon-btn" data-a="link" aria-label="Copy link">${icon('link')}</button>
        <button class="btn danger" data-a="del">Delete</button>
      </div>`);
    s.onclick = (e) => {
      const a = e.target.closest('[data-a]')?.dataset.a;
      if (a === 'zoom') { hideSheet(); zoomTo(it); }
      else if (a === 'link') copyLink();
      else if (a === 'edit') editItem(id);
      else if (a === 'del') confirmDelete(id);
      else if (a === 'folder') openItem(fid, false);
    };
  }

  function confirmDelete(id) {
    const it = sync.items.get(id);
    const s = openSheet(`
      <h2>Delete ${esc(it.name || KIND_LABEL[it.kind] || 'folder')}?</h2>
      <p>This removes it for the whole team.</p>
      <div class="row actions">
        <button class="btn" data-a="no">Cancel</button>
        <button class="btn danger" data-a="yes">Delete</button>
      </div>`);
    s.onclick = async (e) => {
      const a = e.target.closest('[data-a]')?.dataset.a;
      if (a === 'yes') { await sync.deleteItem(id); closeSheet(); }
      else if (a === 'no') openItem(id, false);
    };
  }

  const folderOptions = (selected) => allFolders()
    .map((f) => `<option value="${esc(f.id)}"${f.id === selected ? ' selected' : ''}>${esc(f.name)}</option>`).join('');

  function editItem(id) {
    const it = sync.items.get(id);
    if (!it) return;
    let color = it.color || DEFAULT_COLORS[it.kind];
    const s = openSheet(`
      <h2>${esc(KIND_LABEL[it.kind])}</h2>
      <div class="sub">${esc(describe(it))}</div>
      <label for="f-name">Name</label>
      <input id="f-name" maxlength="80" value="${esc(it.name)}">
      <label for="f-folder">Folder</label>
      <select id="f-folder">${folderOptions(folderOf(it))}</select>
      <label>Color</label>
      <div class="swatches">${COLORS.map((c) => `<button class="swatch${c === color ? ' sel' : ''}" data-c="${c}" style="background:${c}" aria-label="${c}"></button>`).join('')}</div>
      <label for="f-remarks">Remarks</label>
      <textarea id="f-remarks" maxlength="2000">${esc(it.remarks)}</textarea>
      <div class="row actions">
        <button class="btn" data-a="cancel">Close</button>
        <button class="btn primary" data-a="save">Save</button>
      </div>`);
    s.onclick = async (e) => {
      const sw = e.target.closest('[data-c]');
      if (sw) {
        color = sw.dataset.c;
        s.querySelectorAll('.swatch').forEach((b) => b.classList.toggle('sel', b === sw));
        return;
      }
      const a = e.target.closest('[data-a]')?.dataset.a;
      if (a === 'cancel') { if (sync.items.get(id)) openItem(id, false); else closeSheet(); }
      else if (a === 'save') {
        const cur = sync.items.get(id);
        const folder = $('#f-folder').value;
        if (folder !== folderOf(cur) && !folderVisible(folder)) {
          setFolderVisible(folder, true);
          toast(`Moved to ${folderName(folder)} (now shown)`);
        }
        await sync.putItem({ ...cur, name: $('#f-name').value.trim(), remarks: $('#f-remarks').value, color, folder });
        openItem(id, false);
      }
    };
  }

  function zoomTo(it) {
    if (it.kind === 'waypoint') map.setView(it.coords[0], Math.max(map.getZoom(), 15));
    else map.fitBounds(L.latLngBounds(it.coords), { padding: [40, 40] });
  }

  function fitFolder(fid) {
    const pts = objectsIn(fid).flatMap((i) => i.coords);
    if (pts.length === 1) map.setView(pts[0], Math.max(map.getZoom(), 15));
    else if (pts.length) map.fitBounds(L.latLngBounds(pts), { padding: [40, 40], maxZoom: 16 });
  }

  // ----- folder sheets -----
  function showFolder(fid) {
    const f = sync.items.get(fid);
    if (!live(f)) return;
    const owner = personalOwner(fid);
    const objs = objectsIn(fid);
    const isTarget = drawTarget() === fid;
    const vis = folderVisible(fid);
    const canDelete = !owner && objs.length === 0;
    const s = openSheet(`
      <h2>${icon('folder')} ${esc(f.name)}</h2>
      <div class="sub">${owner ? `Personal folder of ${esc(owner.callsign)}` : `Shared folder · created by ${esc(author(f.createdBy))}`}</div>
      <div class="sub">${objs.length} object${objs.length === 1 ? '' : 's'} · ${vis ? 'shown' : 'hidden'} on map${isTarget ? ' · new objects go here' : ''}</div>
      <div class="row actions">
        <button class="btn" data-a="vis">${vis ? 'Hide' : 'Show'}</button>
        ${isTarget ? '' : '<button class="btn" data-a="target">New objects here</button>'}
        ${owner ? '' : '<button class="btn" data-a="rename">Rename</button>'}
        <button class="btn icon-btn" data-a="link" aria-label="Copy link">${icon('link')}</button>
        ${canDelete ? '<button class="btn danger" data-a="del">Delete</button>' : ''}
      </div>
      ${owner || canDelete ? '' : '<p class="sub">Move or delete its objects to delete this folder.</p>'}`);
    s.onclick = (e) => {
      const a = e.target.closest('[data-a]')?.dataset.a;
      if (a === 'vis') { setFolderVisible(fid, !vis); showFolder(fid); }
      else if (a === 'target') {
        prefs.drawFolder = fid;
        savePrefs(prefs);
        if (!vis) setFolderVisible(fid, true);
        toast(`New objects go to ${f.name}`);
        showFolder(fid);
      } else if (a === 'rename') folderForm(fid);
      else if (a === 'link') copyLink();
      else if (a === 'del') confirmDelete(fid);
    };
  }

  // Create (fid undefined) or rename a shared folder.
  function folderForm(fid) {
    const f = fid ? sync.items.get(fid) : null;
    const s = openSheet(`
      <h2>${f ? 'Rename folder' : 'New folder'}</h2>
      <label for="f-fname">Name</label>
      <input id="f-fname" maxlength="80" value="${esc(f?.name ?? '')}" placeholder="e.g. Recon day 2">
      ${f ? '' : '<label><input type="checkbox" id="f-target" checked style="width:auto;min-height:0"> Put my new objects here</label>'}
      <div class="row actions">
        <button class="btn" data-a="cancel">Cancel</button>
        <button class="btn primary" data-a="save">${f ? 'Save' : 'Create'}</button>
      </div>`);
    const input = $('#f-fname');
    input.focus();
    s.onclick = async (e) => {
      const a = e.target.closest('[data-a]')?.dataset.a;
      if (a === 'cancel') { if (f) openItem(fid, false); else closeSheet(); return; }
      if (a !== 'save') return;
      const name = input.value.trim();
      if (!name) { toast('Give the folder a name'); return; }
      if (f) {
        await sync.putItem({ ...f, name });
        openItem(fid, false);
        return;
      }
      const id = newId();
      prefs.folders[id] = true;
      if ($('#f-target').checked) prefs.drawFolder = id;
      savePrefs(prefs);
      await sync.putItem({ id, kind: 'folder', name, coords: [] });
      openItem(id, false);
    };
  }
  const newFolder = () => folderForm();

  function showUser(uid) {
    const p = sync.positions.get(uid);
    const callsign = p?.callsign ?? sync.users.get(uid)?.callsign ?? (uid === me.id ? me.callsign : '?');
    const dist = p && lastFix ? ` · ${formatDistance(L.latLng(lastFix.lat, lastFix.lon).distanceTo([p.lat, p.lon]))} away` : '';
    const fid = sync.users.get(uid)?.folderId;
    const s = openSheet(`
      <h2>${esc(callsign)}</h2>
      ${p ? `<div class="sub">${esc(formatCoord(p.lat, p.lon))}</div>
      <div class="sub">seen ${ago(p.ts)}${dist}${p.acc ? ` · ±${Math.round(p.acc)} m` : ''}</div>`
        : '<div class="sub">No position shared yet</div>'}
      ${fid && live(sync.items.get(fid)) ? `<div class="sub">folder <button class="folder-link" data-a="folder">${esc(folderName(fid))}</button> · ${objectsIn(fid).length} objects</div>` : ''}
      <div class="row actions">
        ${p ? '<button class="btn" data-a="zoom">Zoom</button>' : ''}
        <button class="btn icon-btn" data-a="link" aria-label="Copy link">${icon('link')}</button>
      </div>`);
    s.onclick = (e) => {
      const a = e.target.closest('[data-a]')?.dataset.a;
      if (a === 'zoom') { hideSheet(); setFollow(false); map.setView([p.lat, p.lon], Math.max(map.getZoom(), 15)); }
      else if (a === 'link') copyLink();
      else if (a === 'folder') openItem(fid, false);
    };
  }

  function confirmLeave() {
    const n = sync.pendingCount;
    const s = openSheet(`
      <h2>Leave this team?</h2>
      <p>All local data is removed from this device${n ? `, including <b>${n} change(s) not yet sent</b>` : ''}. You will need a new invite link to come back.</p>
      <div class="row actions">
        <button class="btn" data-a="no">Cancel</button>
        <button class="btn danger" data-a="yes">Leave</button>
      </div>`);
    s.onclick = (e) => {
      const a = e.target.closest('[data-a]')?.dataset.a;
      if (a === 'yes') leave();
      else if (a === 'no') closeSheet();
    };
  }
}

// ---------- helpers ----------

function newId() {
  if (crypto.randomUUID) return crypto.randomUUID();
  // Non-secure contexts lack randomUUID; build a v4 UUID by hand.
  const b = crypto.getRandomValues(new Uint8Array(16));
  b[6] = (b[6] & 0x0f) | 0x40;
  b[8] = (b[8] & 0x3f) | 0x80;
  const h = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

function ago(ts) {
  const s = Math.max(0, Math.round((Date.now() - ts) / 1000));
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.round(s / 60)}m ago`;
  if (s < 86400) return `${Math.round(s / 3600)}h ago`;
  return new Date(ts).toLocaleDateString();
}

let toastTimer;
function toast(msg) {
  const t = $('#toast');
  t.textContent = msg;
  t.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { t.hidden = true; }, 3500);
}

// ---------- boot ----------

if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').catch((e) => console.warn('service worker', e));
}

const params = new URLSearchParams(location.search);
const invite = location.pathname === '/join' ? params.get('t') : null;
const session = loadSession();

if (invite) showJoin(invite, session);
else if (session) startApp(session);
else {
  if (/^\/[iu]\//.test(location.pathname)) {
    $('#noinvite p').textContent = 'This link only opens for members of its team. Join with your team\'s invite link first.';
  }
  $('#noinvite').hidden = false;
}
