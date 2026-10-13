// Leaflet layer drawing the MGRS grid for the current view. The spacing
// follows the zoom: zone/band lines, then 100 km, 10 km, 1 km (no finer).
// With the 10 km and 1 km grids, the km values within the 100 km square are
// printed where lines meet the top edge (eastings) and the left edge
// (northings), as on a printed map; they follow the map while panning.
// The grid is computed for the view plus a margin, and each line is cut
// exactly where it leaves that area or its UTM zone (found by bisection along
// the line), so lines reach the screen edges and meet at zone boundaries.
// Known limitation: the Norway/Svalbard zone exceptions are not drawn.

import { toUTM, fromUTM, centralMeridian, square100k } from './mgrs.js';

const STEPS = 32; // vertices per grid line across the drawn area
const MARGIN = 0.5; // draw this much beyond the view on each side (fraction of its size)

function spacingForZoom(z) {
  if (z >= 13) return 1000;
  if (z >= 10) return 10000;
  if (z >= 7) return 100000;
  return 0; // zone/band lines only
}

export const MgrsGrid = L.LayerGroup.extend({
  options: { color: '#1a1a1a', weight: 1, opacity: 0.55, zoneColor: '#c0392b' },

  onAdd(map) {
    this._map = map;
    if (!map.getPane('mgrs')) {
      const pane = map.createPane('mgrs');
      pane.style.zIndex = 350; // above tiles, below vectors
      pane.style.pointerEvents = 'none';
    }
    this._renderer = L.canvas({ pane: 'mgrs', padding: 0.1 });
    this._edge = L.DomUtil.create('div', 'mgrs-edge', map.getContainer());
    map.on('moveend', this._redraw, this);
    map.on('move zoom resize', this._placeLabels, this);
    this._redraw();
  },

  onRemove(map) {
    map.off('moveend', this._redraw, this);
    map.off('move zoom resize', this._placeLabels, this);
    this._edge?.remove();
    this._edge = null;
    this.clearLayers();
  },

  _line(latlngs, opts) {
    if (latlngs.length > 1) {
      this.addLayer(L.polyline(latlngs, { renderer: this._renderer, interactive: false, ...opts }));
    }
  },

  _redraw() {
    const map = this._map;
    if (!map) return;
    this.clearLayers();
    this._lines = []; // {axis: 'E'|'N', text, pts} for the edge labels
    const b = map.getBounds().pad(MARGIN);
    const south = Math.max(b.getSouth(), -80), north = Math.min(b.getNorth(), 84);
    const west = Math.max(b.getWest(), -180), east = Math.min(b.getEast(), 180);
    if (south >= north) return;
    const z = map.getZoom();
    const zoneStyle = { color: this.options.zoneColor, weight: 2, opacity: 0.8 };

    // Zone boundaries and latitude bands.
    for (let lon = Math.ceil((west + 180) / 6) * 6 - 180; lon <= east; lon += 6) {
      this._line([[south, lon], [north, lon]], zoneStyle);
    }
    for (let lat = Math.ceil((south + 80) / 8) * 8 - 80; lat <= north; lat += 8) {
      this._line([[lat, west], [lat, east]], zoneStyle);
    }

    const sp = spacingForZoom(z);
    if (!sp) { this._placeLabels(); return; }
    const style = { color: this.options.color, weight: sp >= 100000 ? 1.5 : 1, opacity: this.options.opacity };

    const z0 = Math.floor((west + 180) / 6) + 1, z1 = Math.min(Math.floor((east + 180) / 6) + 1, 60);
    for (let zone = z0; zone <= z1; zone++) {
      const cm = centralMeridian(zone);
      const zw = Math.max(west, cm - 3), ze = Math.min(east, cm + 3);
      if (zw >= ze) continue;
      // Split at the equator: northings jump by 10 000 km there.
      if (south < 0 && north > 0) {
        this._zone(zone, south, 0, zw, ze, sp, style, z);
        this._zone(zone, 0, north, zw, ze, sp, style, z);
      } else {
        this._zone(zone, south, north, zw, ze, sp, style, z);
      }
    }
    this._placeLabels();
  },

  // Puts the km labels where grid lines cross the top and left screen edges.
  _placeLabels() {
    const map = this._map, el = this._edge;
    if (!map || !el) return;
    const { x: w, y: h } = map.getSize();
    // Keep clear of the button column (top right) and the status box (bottom left).
    const xMax = w - 80, yMax = h - 170, minGap = 22;
    const html = [];
    let lastX = -1e9, lastY = -1e9;
    const hits = [];
    for (const l of this._lines ?? []) {
      const p = l.pts.map((ll) => map.latLngToContainerPoint(ll));
      for (let i = 1; i < p.length; i++) {
        const [a, b] = [p[i - 1], p[i]];
        if (l.axis === 'E' && (a.y - 0) * (b.y - 0) <= 0 && a.y !== b.y) {
          const x = a.x + (b.x - a.x) * (0 - a.y) / (b.y - a.y);
          if (x > 8 && x < xMax) hits.push({ axis: 'E', pos: x, text: l.text });
        } else if (l.axis === 'N' && (a.x - 0) * (b.x - 0) <= 0 && a.x !== b.x) {
          const y = a.y + (b.y - a.y) * (0 - a.x) / (b.x - a.x);
          if (y > 8 && y < yMax) hits.push({ axis: 'N', pos: y, text: l.text });
        }
      }
    }
    hits.sort((a, b) => a.pos - b.pos);
    for (const t of hits) {
      if (t.axis === 'E') {
        if (t.pos - lastX < minGap) continue;
        lastX = t.pos;
        html.push(`<span class="e" style="left:${t.pos.toFixed(1)}px">${t.text}</span>`);
      } else {
        if (t.pos - lastY < minGap) continue;
        lastY = t.pos;
        html.push(`<span class="n" style="top:${t.pos.toFixed(1)}px">${t.text}</span>`);
      }
    }
    el.innerHTML = html.join('');
  },

  // Draws grid lines for one zone within [s,n] x [w,e] (degrees).
  _zone(zone, s, n, w, e, sp, style, z) {
    const isSouth = s < 0;
    const utm = (lat, lon) => toUTM(isSouth ? Math.min(lat, -1e-9) : lat, lon, zone);
    // Easting/northing extent of the clipped box (sample its edges).
    let minE = Infinity, maxE = -Infinity, minN = Infinity, maxN = -Infinity;
    for (let i = 0; i <= 8; i++) {
      for (const [lat, lon] of [[s + (n - s) * i / 8, w], [s + (n - s) * i / 8, e], [s, w + (e - w) * i / 8], [n, w + (e - w) * i / 8]]) {
        const u = utm(lat, lon);
        minE = Math.min(minE, u.easting); maxE = Math.max(maxE, u.easting);
        minN = Math.min(minN, u.northing); maxN = Math.max(maxN, u.northing);
      }
    }
    if ((maxE - minE) / sp > 200 || (maxN - minN) / sp > 200) return; // too dense

    const inBox = (p) => p.lat >= s - 1e-9 && p.lat <= n + 1e-9 && p.lon >= w - 1e-9 && p.lon <= e + 1e-9;
    // Traces a grid line given by at(v) (v = northing or easting along it) for
    // v in [v0, v1]: samples it, keeps the parts inside the box, and finds the
    // exact point where it enters or leaves by bisecting between samples.
    const trace = (at, v0, v1) => {
      const edge = (vIn, vOut) => {
        for (let k = 0; k < 24; k++) {
          const mid = (vIn + vOut) / 2;
          if (inBox(at(mid))) vIn = mid; else vOut = mid;
        }
        const p = at(vIn);
        return [p.lat, p.lon];
      };
      const runs = [];
      let run = null, prevV = v0, prevIn = false;
      for (let i = 0; i <= STEPS; i++) {
        const v = v0 + (v1 - v0) * i / STEPS;
        const p = at(v);
        const inside = inBox(p);
        if (inside && !prevIn) { run = i ? [edge(v, prevV)] : []; runs.push(run); }
        if (inside) run.push([p.lat, p.lon]);
        if (!inside && prevIn) run.push(edge(prevV, v));
        prevV = v; prevIn = inside;
      }
      return runs;
    };

    // Lines a bit past the extent too, so none stops short at the box edge.
    const e0 = Math.ceil(minE / sp) * sp, n0 = Math.ceil(minN / sp) * sp;
    const lo = (v) => v - (maxE - minE + maxN - minN) * 0.02, hi = (v) => v + (maxE - minE + maxN - minN) * 0.02;
    // km within the 100 km square, two digits: easting 437 000 -> "37".
    const km = (v) => String(Math.round(((v % 100000) + 100000) % 100000 / 1000) % 100).padStart(2, '0');
    const label = sp <= 10000;
    for (let ee = e0; ee <= maxE; ee += sp) {
      for (const r of trace((v) => fromUTM(zone, isSouth, ee, v), lo(minN), hi(maxN))) {
        this._line(r, style);
        if (label) this._lines.push({ axis: 'E', text: km(ee), pts: r });
      }
    }
    for (let nn = n0; nn <= maxN; nn += sp) {
      for (const r of trace((v) => fromUTM(zone, isSouth, v, nn), lo(minE), hi(maxE))) {
        this._line(r, style);
        if (label) this._lines.push({ axis: 'N', text: km(nn), pts: r });
      }
    }

    // Label 100 km squares when they are big on screen.
    if (z >= 7 && z <= 12) {
      const s100 = 100000;
      for (let ee = Math.floor(minE / s100) * s100; ee < maxE; ee += s100) {
        for (let nn = Math.floor(minN / s100) * s100; nn < maxN; nn += s100) {
          const c = fromUTM(zone, isSouth, Math.max(ee, minE) / 2 + Math.min(ee + s100, maxE) / 2,
            Math.max(nn, minN) / 2 + Math.min(nn + s100, maxN) / 2);
          const id = square100k(zone, ee + 1, nn + 1);
          if (!id || !inBox(c)) continue;
          this.addLayer(L.marker([c.lat, c.lon], {
            interactive: false, pane: 'mgrs',
            icon: L.divIcon({ className: 'mgrs-label', html: id, iconSize: null }),
          }));
        }
      }
    }
  },
});
