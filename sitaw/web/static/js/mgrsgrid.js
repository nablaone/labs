// Leaflet layer drawing the MGRS grid for the current view. The spacing
// follows the zoom: zone/band lines, then 100 km, 10 km, 1 km, 100 m.
// Known limitation: the Norway/Svalbard zone exceptions are not drawn.

import { toUTM, fromUTM, centralMeridian, square100k } from './mgrs.js';

const STEPS = 24; // vertices per grid line inside a view

function spacingForZoom(z) {
  if (z >= 16) return 100;
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
    map.on('moveend', this._redraw, this);
    this._redraw();
  },

  onRemove(map) {
    map.off('moveend', this._redraw, this);
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
    const b = map.getBounds();
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
    if (!sp) return;
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
    const clipped = (pts) => {
      // Keep the runs of points inside the box; good enough at STEPS resolution.
      const runs = [[]];
      for (const p of pts) {
        if (inBox(p)) runs[runs.length - 1].push([p.lat, p.lon]);
        else if (runs[runs.length - 1].length) runs.push([]);
      }
      return runs;
    };

    const e0 = Math.ceil(minE / sp) * sp, n0 = Math.ceil(minN / sp) * sp;
    const dN = (maxN - minN) / STEPS, dE = (maxE - minE) / STEPS;
    for (let ee = e0; ee <= maxE; ee += sp) {
      const pts = [];
      for (let i = 0; i <= STEPS; i++) pts.push(fromUTM(zone, isSouth, ee, minN + dN * i));
      for (const run of clipped(pts)) this._line(run, style);
    }
    for (let nn = n0; nn <= maxN; nn += sp) {
      const pts = [];
      for (let i = 0; i <= STEPS; i++) pts.push(fromUTM(zone, isSouth, minE + dE * i, nn));
      for (const run of clipped(pts)) this._line(run, style);
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
