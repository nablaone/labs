// Tap-to-place drawing for waypoints, lines and areas. Vertices come from map
// taps or from the center crosshair ("✛ Add"), which is easier on a phone.

const MIN_POINTS = { waypoint: 1, line: 2, area: 3 };

export class DrawTool {
  constructor(map, { onFinish, onChange }) {
    this.map = map;
    this.onFinish = onFinish;
    this.onChange = onChange;
    this.kind = null;
    this.pts = [];
    this.preview = L.layerGroup();
    this._click = (e) => this.add(e.latlng);
  }

  get active() {
    return this.kind !== null;
  }

  start(kind) {
    this.cancel();
    this.kind = kind;
    this.pts = [];
    this.preview.addTo(this.map);
    this.map.on('click', this._click);
    this.changed();
  }

  add(latlng) {
    if (!this.active) return;
    this.pts.push([latlng.lat, latlng.lng]);
    if (this.kind === 'waypoint') {
      this.finish();
      return;
    }
    this.redraw();
  }

  addCenter() {
    this.add(this.map.getCenter());
  }

  undo() {
    this.pts.pop();
    this.redraw();
  }

  cancel() {
    if (!this.active) return;
    this.map.off('click', this._click);
    this.preview.clearLayers();
    this.preview.remove();
    this.kind = null;
    this.pts = [];
    this.changed();
  }

  // Returns false when there aren't enough vertices yet.
  finish() {
    if (!this.active) return false;
    if (this.pts.length < MIN_POINTS[this.kind]) return false;
    const kind = this.kind, coords = this.pts;
    this.pts = [];
    this.cancel();
    this.onFinish(kind, coords);
    return true;
  }

  redraw() {
    this.preview.clearLayers();
    const style = { color: '#ffeb3b', weight: 4, dashArray: '8 6', interactive: false };
    if (this.pts.length > 1) {
      (this.kind === 'area' ? L.polygon(this.pts, { ...style, fillOpacity: 0.15 }) : L.polyline(this.pts, style)).addTo(this.preview);
    }
    for (const p of this.pts) {
      L.circleMarker(p, { radius: 7, color: '#000', weight: 2, fillColor: '#ffeb3b', fillOpacity: 1, interactive: false }).addTo(this.preview);
    }
    this.changed();
  }

  changed() {
    this.onChange?.(this.kind, this.pts.length, MIN_POINTS[this.kind] ?? 0);
  }
}
