// Keeps the local replica in sync with the server over a WebSocket.
//
// Writes go to the local replica and an IndexedDB outbox first, so the app
// works offline; the outbox is flushed whenever the socket (re)connects and
// entries are removed when the server acks them. Conflicts are last-write-wins
// on (updatedAt, updatedBy), the same rule the server uses.
//
// Events (CustomEvent.detail): status {online}, reset {items, source},
// item {item}, pos {pos}, positions {positions}, user {user}, leave {userId},
// you {user, team}, unauthorized, rejected {id, error}.
// reset.source is 'cache' (local replica at startup) or 'server' (snapshot).

import * as db from './db.js';

const newer = (a, b) => !b || a.updatedAt > b.updatedAt || (a.updatedAt === b.updatedAt && a.updatedBy > b.updatedBy);

export class Sync extends EventTarget {
  constructor(token, userId, teamId) {
    super();
    this.token = token;
    this.userId = userId;
    this.teamId = teamId; // undefined for sessions created before teams existed
    this.items = new Map();
    this.positions = new Map();
    this.users = new Map(); // team roster from the last snapshot
    this.outbox = new Map();
    this.online = false;
    this.pendingPos = null;
    this.backoff = 1000;
    this.clockOffset = 0; // server time - local time, ms
  }

  now() {
    return Date.now() + this.clockOffset;
  }

  emit(type, detail) {
    this.dispatchEvent(new CustomEvent(type, { detail }));
  }

  async start() {
    // A broken local cache must never keep us from connecting.
    try {
      // The cache belongs to one team; never show another team's data from it.
      const cachedTeam = await db.get('kv', 'team');
      if (cachedTeam !== this.teamId) {
        await db.clear('items');
        await db.clear('outbox');
        await db.del('kv', 'positions');
        if (this.teamId) await db.put('kv', 'team', this.teamId);
      }
      for (const it of await db.all('items')) this.items.set(it.id, it);
      for (const it of await db.all('outbox')) this.outbox.set(it.id, it);
      for (const p of (await db.get('kv', 'positions')) ?? []) this.positions.set(p.userId, p);
    } catch (e) {
      console.error('loading local cache failed', e);
    }
    this.emit('reset', { items: this.liveItems(), source: 'cache' });
    this.emit('positions', { positions: [...this.positions.values()] });
    this.connect();
    window.addEventListener('online', () => this.reconnectNow());
  }

  liveItems() {
    return [...this.items.values()].filter((i) => !i.deleted);
  }

  connect() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${proto}//${location.host}/ws?token=${encodeURIComponent(this.token)}`);
    this.ws = ws;
    let opened = false;
    ws.onopen = () => {
      opened = true;
      this.backoff = 1000;
      this.setOnline(true);
    };
    ws.onmessage = (ev) => {
      let m;
      try { m = JSON.parse(ev.data); } catch { return; }
      this.handle(m);
    };
    ws.onclose = async () => {
      if (this.ws !== ws) return;
      this.setOnline(false);
      if (!opened && navigator.onLine && await this.isUnauthorized()) {
        this.emit('unauthorized');
        return;
      }
      clearTimeout(this.retry);
      this.retry = setTimeout(() => this.connect(), this.backoff);
      this.backoff = Math.min(this.backoff * 2, 30000);
    };
  }

  reconnectNow() {
    if (this.online) return;
    clearTimeout(this.retry);
    this.backoff = 1000;
    this.ws?.close();
    this.connect();
  }

  async isUnauthorized() {
    try {
      const r = await fetch('/api/me', { headers: { Authorization: `Bearer ${this.token}` } });
      return r.status === 401;
    } catch {
      return false;
    }
  }

  setOnline(v) {
    this.online = v;
    this.emit('status', { online: v });
  }

  send(m) {
    if (this.online && this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(m));
      return true;
    }
    return false;
  }

  async handle(m) {
    switch (m.t) {
      case 'snapshot': {
        if (m.now) this.clockOffset = m.now - Date.now();
        this.users = new Map((m.users ?? []).map((u) => [u.id, u]));
        if (m.team && m.team.id !== this.teamId) {
          // A pre-teams session just learns its team (its outbox is its own);
          // a real team change drops edits queued for the other team.
          if (this.teamId) {
            this.outbox.clear();
            await db.clear('outbox');
          }
          this.teamId = m.team.id;
          await db.put('kv', 'team', m.team.id);
        }
        this.emit('you', { user: m.you, team: m.team });
        // Server state is authoritative except where our outbox holds a newer edit.
        const next = new Map();
        for (const it of m.items ?? []) next.set(it.id, it);
        for (const [id, it] of this.outbox) if (newer(it, next.get(id))) next.set(id, it);
        this.items = next;
        await db.clear('items');
        for (const it of next.values()) await db.put('items', it.id, it);
        this.positions = new Map((m.positions ?? []).map((p) => [p.userId, p]));
        this.savePositions();
        this.emit('positions', { positions: [...this.positions.values()] });
        this.emit('reset', { items: this.liveItems(), source: 'server' });

        for (const it of this.outbox.values()) this.send({ t: 'put', item: it });
        if (this.pendingPos) this.sendPos(this.pendingPos);
        break;
      }
      case 'item':
        this.applyRemote(m.item);
        break;
      case 'ack': {
        const pending = this.outbox.get(m.id);
        // A refused change (e.g. deleting a non-empty folder) comes back with the
        // server copy, which must replace our newer-looking local edit.
        if (m.error && m.item) this.applyRemote(m.item, true);
        else if (m.item) this.applyRemote(m.item);
        // Done with the outbox entry once the server has processed that exact
        // version (sentAt) or holds something at least as new.
        if (pending && (m.error || pending.updatedAt === m.sentAt || (m.item && !newer(pending, m.item)))) {
          this.outbox.delete(m.id);
          await db.del('outbox', m.id);
        }
        if (m.error) this.emit('rejected', { id: m.id, error: m.error });
        break;
      }
      case 'pos':
        this.positions.set(m.pos.userId, m.pos);
        if (!this.users.has(m.pos.userId)) this.users.set(m.pos.userId, { id: m.pos.userId, callsign: m.pos.callsign });
        this.savePositions();
        this.emit('pos', { pos: m.pos });
        break;
      case 'user':
        this.users.set(m.user.id, m.user);
        this.emit('user', { user: m.user });
        break;
      case 'leave':
        this.positions.delete(m.userId);
        this.users.delete(m.userId);
        this.savePositions();
        this.emit('leave', { userId: m.userId });
        break;
    }
  }

  applyRemote(it, force = false) {
    if (!force) {
      if (!newer(it, this.items.get(it.id))) return;
      if (this.outbox.has(it.id) && newer(this.outbox.get(it.id), it)) return;
    }
    this.items.set(it.id, it);
    db.put('items', it.id, it);
    this.emit('item', { item: it });
  }

  // Creates, updates or (with deleted: true) deletes an item.
  async putItem(item) {
    const prev = this.items.get(item.id);
    const it = {
      ...item,
      createdBy: prev?.createdBy ?? this.userId,
      updatedBy: this.userId,
      updatedAt: Math.max(this.now(), (prev?.updatedAt ?? 0) + 1),
    };
    this.items.set(it.id, it);
    this.outbox.set(it.id, it);
    this.emit('item', { item: it });
    await db.put('items', it.id, it);
    await db.put('outbox', it.id, it);
    this.send({ t: 'put', item: it });
    return it;
  }

  deleteItem(id) {
    const prev = this.items.get(id);
    if (prev) return this.putItem({ id, kind: prev.kind, deleted: true });
  }

  // Only the latest fix matters; offline fixes are not queued.
  sendPos(pos) {
    this.pendingPos = this.send({ t: 'pos', pos }) ? null : pos;
  }

  savePositions() {
    clearTimeout(this.posSave);
    this.posSave = setTimeout(() => db.put('kv', 'positions', [...this.positions.values()]), 2000);
  }

  get pendingCount() {
    return this.outbox.size;
  }
}
