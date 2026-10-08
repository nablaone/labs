// Rendering of shared items and team positions on the Leaflet map.

import { formatDistance, formatArea, formatBearing, bearingGrid } from './coords.js';

export const DEFAULT_COLORS = { waypoint: '#e53935', line: '#1e88e5', area: '#fb8c00' };
const STALE_MS = 5 * 60 * 1000;
const LOST_MS = 30 * 60 * 1000;

const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);

// Draws map objects (folders and positions are drawn elsewhere). visible(item)
// decides what is shown, e.g. by folder; call reset() when it changes.
//
// Waypoint: a diamond standing on a small circle; the circle is the point.
// Line: dot at the start (name label there), arrowhead at the end, and per
// segment "length · grid bearing" along the segment once zoomed in.
// Area: name at the centre, its area below once the shape is big on screen.
const DETAIL_ZOOM = 13;      // a few km across on a phone
const SEG_LABEL_MIN_PX = 110; // segment must be this long on screen to carry a label
const AREA_LABEL_MIN_PX = 90; // shape must be this big on screen to show its area

export class ItemsLayer {
  constructor(map, onSelect, visible = () => true) {
    this.map = map;
    this.onSelect = onSelect;
    this.visible = visible;
    this.groups = {
      area: L.layerGroup().addTo(map),
      line: L.layerGroup().addTo(map),
      waypoint: L.layerGroup().addTo(map),
    };
    this.layers = new Map(); // id -> leaflet layer
    this.details = new Map(); // id -> () => void, refreshes zoom-dependent labels
    map.on('zoomend', () => { for (const f of this.details.values()) f(); });
  }

  reset(items) {
    for (const g of Object.values(this.groups)) g.clearLayers();
    this.layers.clear();
    this.details.clear();
    for (const it of items) this.upsert(it);
  }

  remove(id) {
    const l = this.layers.get(id);
    if (l) {
      for (const g of Object.values(this.groups)) g.removeLayer(l);
      this.layers.delete(id);
      this.details.delete(id);
    }
  }

  upsert(it) {
    this.remove(it.id);
    if (it.deleted || !this.groups[it.kind] || !this.visible(it)) return;
    const color = it.color || DEFAULT_COLORS[it.kind];
    const ll = it.coords.map(([lat, lon]) => [lat, lon]);
    const label = (layer, direction, offset) => it.name &&
      layer.bindTooltip(esc(it.name), { permanent: true, direction, offset, className: 'map-label' });
    let layer;
    if (it.kind === 'waypoint') {
      layer = L.marker(ll[0], { icon: waypointIcon(color) });
      label(layer, 'right', [14, -13]);
    } else if (it.kind === 'line') {
      layer = this.line(it, ll, color, label);
    } else {
      layer = L.polygon(ll, { color, weight: 4, fillOpacity: 0.2 });
      this.details.set(it.id, () => this.areaLabel(it, layer));
    }
    layer.on('click', (e) => {
      L.DomEvent.stopPropagation(e);
      this.onSelect(it.id, e.latlng);
    });
    layer.addTo(this.groups[it.kind]);
    this.layers.set(it.id, layer);
    this.details.get(it.id)?.();
  }

  // A line: the path, a start dot carrying the name, an arrowhead at the end,
  // and one "length · bearing" label per segment (shown when zoomed in).
  line(it, ll, color, label) {
    const g = L.featureGroup();
    L.polyline(ll, { color, weight: 5 }).addTo(g);
    const start = L.circleMarker(ll[0], { radius: 6, weight: 2, color: '#000', fillColor: color, fillOpacity: 1 }).addTo(g);
    label(start, 'right', [10, 0]);
    const n = ll.length;
    L.marker(ll[n - 1], { icon: arrowIcon(color, this.screenAngle(ll[n - 2], ll[n - 1])), keyboard: false }).addTo(g);

    const segs = [];
    for (let i = 1; i < n; i++) {
      const [a, b] = [ll[i - 1], ll[i]];
      const text = `${formatDistance(L.latLng(a).distanceTo(b))} · ${formatBearing(bearingGrid(a[0], a[1], b[0], b[1]))}`;
      const mid = [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2];
      const m = L.marker(mid, { icon: segLabelIcon(text, this.screenAngle(a, b)), interactive: false, keyboard: false });
      segs.push({ m, a, b });
    }
    this.details.set(it.id, () => {
      for (const s of segs) {
        const px = this.map.latLngToLayerPoint(s.a).distanceTo(this.map.latLngToLayerPoint(s.b));
        const show = this.map.getZoom() >= DETAIL_ZOOM && px >= SEG_LABEL_MIN_PX;
        if (show && !g.hasLayer(s.m)) s.m.addTo(g);
        else if (!show && g.hasLayer(s.m)) g.removeLayer(s.m);
      }
    });
    return g;
  }

  // Name in the middle; the area just below it when the shape is big enough.
  areaLabel(it, layer) {
    const b = layer.getBounds();
    const nw = this.map.latLngToLayerPoint(b.getNorthWest()), se = this.map.latLngToLayerPoint(b.getSouthEast());
    const big = Math.min(se.x - nw.x, se.y - nw.y) >= AREA_LABEL_MIN_PX;
    const html = [it.name && esc(it.name), big && `<span class="area-size">${formatArea(polygonArea(it.coords))}</span>`]
      .filter(Boolean).join('');
    if (!html) { layer.unbindTooltip(); return; }
    if (layer.getTooltip()) layer.setTooltipContent(html);
    else layer.bindTooltip(html, { permanent: true, direction: 'center', className: 'map-label' });
  }

  // Direction from a to b on screen, in degrees (Web Mercator is conformal,
  // so this does not change with zoom).
  screenAngle(a, b) {
    const p = this.map.project(a, 0), q = this.map.project(b, 0);
    return Math.atan2(q.y - p.y, q.x - p.x) * 180 / Math.PI;
  }
}

// Diamond above, small circle at its lower corner: the circle is the point.
function waypointIcon(color) {
  return L.divIcon({
    className: '', iconSize: [28, 32], iconAnchor: [14, 27],
    html: `<svg class="wp-svg" width="28" height="32" viewBox="0 0 28 32" aria-hidden="true">
      <path d="M14 1 27 14 14 27 1 14Z" fill="${color}" stroke="#000" stroke-width="2.5" stroke-linejoin="miter"/>
      <circle cx="14" cy="27" r="4" fill="#fff" stroke="#000" stroke-width="2"/></svg>`,
  });
}

// Arrowhead whose tip sits exactly on the line's last point.
function arrowIcon(color, angle) {
  return L.divIcon({
    className: '', iconSize: [18, 16], iconAnchor: [18, 8],
    html: `<svg width="18" height="16" viewBox="0 0 18 16" style="transform-origin:18px 8px;transform:rotate(${angle}deg)" aria-hidden="true">
      <path d="M0 0 18 8 0 16 4 8Z" fill="${color}" stroke="#000" stroke-width="1.5" stroke-linejoin="miter"/></svg>`,
  });
}

// Text centred on a segment, rotated along it (kept upright), just above it.
function segLabelIcon(text, angle) {
  const upright = angle > 90 || angle < -90 ? angle + 180 : angle;
  return L.divIcon({
    className: '', iconSize: [0, 0], iconAnchor: [0, 0],
    html: `<div class="seg-label" style="transform:translate(-50%,-50%) rotate(${upright}deg) translateY(-13px)">${esc(text)}</div>`,
  });
}

// One mark per team member with a position: a bold dot plus a callsign label.
// Old positions stay fully visible; their age goes in the label and the dot
// turns grey (stale > 5 min) or grey and dashed (lost > 30 min).
export class TeamLayer {
  constructor(map, selfId, onSelect) {
    this.map = map;
    this.selfId = selfId;
    this.onSelect = onSelect;
    this.group = L.layerGroup().addTo(map);
    this.markers = new Map(); // userId -> L.marker
    this.positions = new Map();
    setInterval(() => this.restyleAll(), 30000);
  }

  set(p) {
    if (p.userId === this.selfId) return; // self is drawn by SelfMarker
    this.positions.set(p.userId, p);
    let m = this.markers.get(p.userId);
    if (!m) {
      m = L.marker([p.lat, p.lon], { icon: memberIcon('') })
        .bindTooltip('', { permanent: true, direction: 'right', offset: [14, 0], className: 'map-label member-label' })
        .on('click', (e) => { L.DomEvent.stopPropagation(e); this.onSelect(p.userId); })
        .addTo(this.group);
      this.markers.set(p.userId, m);
    }
    m.setLatLng([p.lat, p.lon]);
    this.restyle(p.userId);
  }

  remove(id) {
    this.markers.get(id)?.remove();
    this.markers.delete(id);
    this.positions.delete(id);
  }

  reset(positions) {
    this.group.clearLayers();
    this.markers.clear();
    this.positions.clear();
    for (const p of positions) this.set(p);
  }

  restyle(id) {
    const p = this.positions.get(id), m = this.markers.get(id);
    const age = Date.now() - p.ts;
    const manual = p.source === 'manual';
    // A hand-placed position doesn't go stale like a GPS fix; it says "manual".
    const state = manual ? 'manual' : age > LOST_MS ? 'lost' : age > STALE_MS ? 'stale' : '';
    m.setIcon(memberIcon(state));
    const note = manual ? 'manual' : state ? shortAge(age) : '';
    m.setTooltipContent(esc(p.callsign) + (note ? ` <span class="age">· ${note}</span>` : ''));
  }

  restyleAll() {
    for (const id of this.markers.keys()) this.restyle(id);
  }
}

function memberIcon(state) {
  return L.divIcon({ className: '', html: `<div class="member-icon ${state}"></div>`, iconSize: [24, 24], iconAnchor: [12, 12] });
}

function shortAge(ms) {
  const m = Math.round(ms / 60000);
  return m < 60 ? `${m}m` : m < 1440 ? `${Math.round(m / 60)}h` : `${Math.round(m / 1440)}d`;
}

// Your own position, labelled with your callsign like everyone else.
export class SelfMarker {
  constructor(map, callsign) {
    this.callsign = callsign;
    this.acc = L.circle([0, 0], { radius: 1, color: '#1e88e5', weight: 1, fillOpacity: 0.1, interactive: false });
    this.marker = L.marker([0, 0], {
      icon: L.divIcon({ className: '', html: '<div class="self-icon"></div>', iconSize: [26, 26], iconAnchor: [13, 13] }),
      interactive: false, zIndexOffset: 1000,
    }).bindTooltip(esc(callsign), { permanent: true, direction: 'right', offset: [14, 0], className: 'map-label self-label' });
    this.map = map;
    this.shown = false;
  }

  // manual: placed by hand (hollow dot, "manual" on the label, no accuracy ring).
  update(lat, lon, acc, manual = false) {
    this.marker.setLatLng([lat, lon]);
    this.acc.setLatLng([lat, lon]).setRadius(manual ? 0 : acc || 1);
    this.marker.setTooltipContent(esc(this.callsign) + (manual ? ' <span class="age">· manual</span>' : ''));
    if (!this.shown) {
      this.acc.addTo(this.map);
      this.marker.addTo(this.map);
      this.shown = true;
    }
    this.marker.getElement()?.querySelector('.self-icon')?.classList.toggle('manual', manual);
  }

  // Location unknown: nothing to show.
  hide() {
    this.acc.remove();
    this.marker.remove();
    this.shown = false;
  }
}

// Length of a polyline in meters.
export function lineLength(coords) {
  let d = 0;
  for (let i = 1; i < coords.length; i++) d += L.latLng(coords[i - 1]).distanceTo(L.latLng(coords[i]));
  return d;
}

// Approximate geodesic polygon area in m² (spherical excess method).
export function polygonArea(coords) {
  const R = 6378137, rad = Math.PI / 180;
  let s = 0;
  for (let i = 0; i < coords.length; i++) {
    const [lat1, lon1] = coords[i], [lat2, lon2] = coords[(i + 1) % coords.length];
    s += (lon2 - lon1) * rad * (2 + Math.sin(lat1 * rad) + Math.sin(lat2 * rad));
  }
  return Math.abs(s * R * R / 2);
}
