// Service worker: offline app shell + map tile cache.
//  - App shell: network-first with a short timeout, cache fallback when offline.
//  - Tiles: cache-first, capped at MAX_TILES entries (oldest evicted first).
//  - API and WebSocket traffic is never cached.
// Bump VERSION when the shell file list changes.

const VERSION = 'v8';
const SHELL_CACHE = `sitaw-shell-${VERSION}`;
const TILE_CACHE = 'sitaw-tiles';
const MAX_TILES = 4000;
const TILE_HOSTS = /(^|\.)(tile\.opentopomap\.org|tile\.openstreetmap\.org)$/;

const SHELL = [
  '/', '/css/app.css', '/manifest.webmanifest', '/icon.svg',
  '/js/app.js', '/js/sync.js', '/js/db.js', '/js/layers.js', '/js/draw.js',
  '/js/mgrs.js', '/js/mgrsgrid.js', '/js/coords.js', '/js/icons.js',
  '/vendor/leaflet/leaflet.js', '/vendor/leaflet/leaflet.css',
  '/vendor/leaflet/images/layers.png', '/vendor/leaflet/images/layers-2x.png',
];

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(SHELL_CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (e) => {
  e.waitUntil((async () => {
    for (const k of await caches.keys()) {
      if (k.startsWith('sitaw-shell-') && k !== SHELL_CACHE) await caches.delete(k);
    }
    await self.clients.claim();
  })());
});

self.addEventListener('fetch', (e) => {
  const req = e.request;
  if (req.method !== 'GET') return;
  const url = new URL(req.url);

  if (TILE_HOSTS.test(url.hostname)) {
    e.respondWith(tile(req));
    return;
  }
  if (url.origin !== location.origin || url.pathname.startsWith('/api/') || url.pathname === '/ws') return;

  // SPA routes (/, /join?t=...) all map to the cached index.
  const key = req.mode === 'navigate' ? '/' : url.pathname;
  e.respondWith(shell(req, key));
});

const NETWORK_TIMEOUT = 3000;

async function shell(req, key) {
  const cache = await caches.open(SHELL_CACHE);
  try {
    const res = await Promise.race([
      fetch(req),
      new Promise((_, reject) => setTimeout(() => reject(new Error('timeout')), NETWORK_TIMEOUT)),
    ]);
    if (res.ok) cache.put(key, res.clone());
    return res;
  } catch {
    return (await cache.match(key)) || new Response('offline', { status: 503 });
  }
}

async function tile(req) {
  const cache = await caches.open(TILE_CACHE);
  // Subdomains (a/b/c) serve the same tile; normalize so they share entries.
  const key = req.url.replace(/\/\/[abc]\./, '//');
  const hit = await cache.match(key);
  if (hit) return hit;
  try {
    const res = await fetch(req);
    if (res.ok) {
      await cache.put(key, res.clone());
      trim(cache);
    }
    return res;
  } catch {
    return new Response('', { status: 504 });
  }
}

let trimming = false;
async function trim(cache) {
  if (trimming) return;
  trimming = true;
  try {
    const keys = await cache.keys();
    for (let i = 0; i < keys.length - MAX_TILES; i++) await cache.delete(keys[i]);
  } finally {
    trimming = false;
  }
}
