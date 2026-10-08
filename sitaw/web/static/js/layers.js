// Rendering of shared items and team positions on the Leaflet map.

export const DEFAULT_COLORS = { waypoint: '#e53935', line: '#1e88e5', area: '#fb8c00' };
const STALE_MS = 5 * 60 * 1000;
const LOST_MS = 30 * 60 * 1000;

const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);

// Draws map objects (folders themselves have no geometry). visible(item)
// decides what is shown, e.g. by folder; call reset() when it changes.
export class ItemsLayer {
  constructor(map, onSelect, visible = () => true) {
    this.onSelect = onSelect;
    this.visible = visible;
    this.groups = {
      area: L.layerGroup().addTo(map),
      line: L.layerGroup().addTo(map),
      waypoint: L.layerGroup().addTo(map),
    };
    this.layers = new Map(); // id -> leaflet layer
  }

  reset(items) {
    for (const g of Object.values(this.groups)) g.clearLayers();
    this.layers.clear();
    for (const it of items) this.upsert(it);
  }

  remove(id) {
    const l = this.layers.get(id);
    if (l) {
      for (const g of Object.values(this.groups)) g.removeLayer(l);
      this.layers.delete(id);
    }
  }

  upsert(it) {
    this.remove(it.id);
    if (it.deleted || !this.groups[it.kind] || !this.visible(it)) return;
    const color = it.color || DEFAULT_COLORS[it.kind];
    const ll = it.coords.map(([lat, lon]) => [lat, lon]);
    let layer;
    if (it.kind === 'waypoint') {
      layer = L.marker(ll[0], {
        icon: L.divIcon({ className: '', html: `<div class="wp-icon" style="background:${color}"></div>`, iconSize: [28, 28], iconAnchor: [14, 14] }),
      });
    } else if (it.kind === 'line') {
      layer = L.polyline(ll, { color, weight: 5 });
    } else {
      layer = L.polygon(ll, { color, weight: 4, fillOpacity: 0.2 });
    }
    if (it.name) {
      layer.bindTooltip(esc(it.name), { permanent: true, direction: it.kind === 'waypoint' ? 'right' : 'center', offset: it.kind === 'waypoint' ? [14, 0] : [0, 0], className: 'map-label' });
    }
    layer.on('click', (e) => {
      L.DomEvent.stopPropagation(e);
      this.onSelect(it.id, e.latlng);
    });
    layer.addTo(this.groups[it.kind]);
    this.layers.set(it.id, layer);
  }
}

export class TeamLayer {
  constructor(map, selfId, onSelect) {
    this.map = map;
    this.selfId = selfId;
    this.onSelect = onSelect;
    this.group = L.layerGroup().addTo(map);
    this.markers = new Map(); // userId -> circleMarker
    this.positions = new Map();
    setInterval(() => this.restyleAll(), 30000);
  }

  set(p) {
    if (p.userId === this.selfId) return; // self is drawn by SelfMarker
    this.positions.set(p.userId, p);
    let m = this.markers.get(p.userId);
    if (!m) {
      m = L.circleMarker([p.lat, p.lon], { radius: 11, weight: 3, color: '#fff', fillColor: '#00acc1', fillOpacity: 1 })
        .bindTooltip(esc(p.callsign), { permanent: true, direction: 'right', offset: [12, 0], className: 'map-label' })
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
    m.setStyle(age > LOST_MS ? { fillOpacity: 0.25, opacity: 0.4, dashArray: '4 4' }
      : age > STALE_MS ? { fillOpacity: 0.5, opacity: 0.7, dashArray: null }
        : { fillOpacity: 1, opacity: 1, dashArray: null });
  }

  restyleAll() {
    for (const id of this.markers.keys()) this.restyle(id);
  }
}

export class SelfMarker {
  constructor(map) {
    this.acc = L.circle([0, 0], { radius: 1, color: '#1e88e5', weight: 1, fillOpacity: 0.1, interactive: false });
    this.marker = L.marker([0, 0], {
      icon: L.divIcon({ className: '', html: '<div class="self-icon"></div>', iconSize: [26, 26], iconAnchor: [13, 13] }),
      interactive: false, zIndexOffset: 1000,
    });
    this.map = map;
    this.shown = false;
  }

  update(lat, lon, acc) {
    this.marker.setLatLng([lat, lon]);
    this.acc.setLatLng([lat, lon]).setRadius(acc || 1);
    if (!this.shown) {
      this.acc.addTo(this.map);
      this.marker.addTo(this.map);
      this.shown = true;
    }
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
