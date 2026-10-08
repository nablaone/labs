// WGS84 lat/lon <-> UTM <-> MGRS. Standard USGS series formulas, sub-meter
// accurate inside a zone. Polar UPS areas (lat < -80 or > 84) are not supported.

const A = 6378137.0;
const E2 = 0.00669438;          // eccentricity squared
const EP2 = E2 / (1 - E2);       // second eccentricity squared
const K0 = 0.9996;
const D2R = Math.PI / 180;

const BANDS = 'CDEFGHJKLMNPQRSTUVWXX';        // 8° bands from -80; X is 12°
const COL_SETS = ['STUVWXYZ', 'ABCDEFGH', 'JKLMNPQR'];  // indexed by zone % 3
const ROWS = 'ABCDEFGHJKLMNPQRSTUV';

export function utmZone(lat, lon) {
  lon = ((lon + 180) % 360 + 360) % 360 - 180;
  let zone = Math.floor((lon + 180) / 6) + 1;
  if (zone > 60) zone = 60;
  // Norway and Svalbard exceptions.
  if (lat >= 56 && lat < 64 && lon >= 3 && lon < 12) zone = 32;
  if (lat >= 72 && lat < 84) {
    if (lon >= 0 && lon < 9) zone = 31;
    else if (lon >= 9 && lon < 21) zone = 33;
    else if (lon >= 21 && lon < 33) zone = 35;
    else if (lon >= 33 && lon < 42) zone = 37;
  }
  return zone;
}

export function centralMeridian(zone) {
  return (zone - 1) * 6 - 180 + 3;
}

export function latBand(lat) {
  if (lat < -80 || lat > 84) return null;
  return BANDS[Math.min(Math.floor((lat + 80) / 8), 20)];
}

// Converts lat/lon to UTM. A zone may be forced (used by the grid overlay to
// project points just outside a zone's edge).
export function toUTM(lat, lon, zone = utmZone(lat, lon)) {
  const phi = lat * D2R;
  const lam = lon * D2R;
  const lam0 = centralMeridian(zone) * D2R;
  const sin = Math.sin(phi), cos = Math.cos(phi), tan = Math.tan(phi);

  const N = A / Math.sqrt(1 - E2 * sin * sin);
  const T = tan * tan;
  const C = EP2 * cos * cos;
  const Aa = cos * (lam - lam0);
  const M = A * (
    (1 - E2 / 4 - 3 * E2 * E2 / 64 - 5 * E2 ** 3 / 256) * phi
    - (3 * E2 / 8 + 3 * E2 * E2 / 32 + 45 * E2 ** 3 / 1024) * Math.sin(2 * phi)
    + (15 * E2 * E2 / 256 + 45 * E2 ** 3 / 1024) * Math.sin(4 * phi)
    - (35 * E2 ** 3 / 3072) * Math.sin(6 * phi));

  const easting = K0 * N * (Aa + (1 - T + C) * Aa ** 3 / 6
    + (5 - 18 * T + T * T + 72 * C - 58 * EP2) * Aa ** 5 / 120) + 500000;
  let northing = K0 * (M + N * tan * (Aa * Aa / 2
    + (5 - T + 9 * C + 4 * C * C) * Aa ** 4 / 24
    + (61 - 58 * T + T * T + 600 * C - 330 * EP2) * Aa ** 6 / 720));
  if (lat < 0) northing += 10000000;
  return { zone, south: lat < 0, easting, northing };
}

export function fromUTM(zone, south, easting, northing) {
  const e1 = (1 - Math.sqrt(1 - E2)) / (1 + Math.sqrt(1 - E2));
  const x = easting - 500000;
  const y = south ? northing - 10000000 : northing;

  const M = y / K0;
  const mu = M / (A * (1 - E2 / 4 - 3 * E2 * E2 / 64 - 5 * E2 ** 3 / 256));
  const phi1 = mu
    + (3 * e1 / 2 - 27 * e1 ** 3 / 32) * Math.sin(2 * mu)
    + (21 * e1 * e1 / 16 - 55 * e1 ** 4 / 32) * Math.sin(4 * mu)
    + (151 * e1 ** 3 / 96) * Math.sin(6 * mu);

  const sin = Math.sin(phi1), cos = Math.cos(phi1), tan = Math.tan(phi1);
  const N1 = A / Math.sqrt(1 - E2 * sin * sin);
  const T1 = tan * tan;
  const C1 = EP2 * cos * cos;
  const R1 = A * (1 - E2) / Math.pow(1 - E2 * sin * sin, 1.5);
  const D = x / (N1 * K0);

  const lat = phi1 - (N1 * tan / R1) * (D * D / 2
    - (5 + 3 * T1 + 10 * C1 - 4 * C1 * C1 - 9 * EP2) * D ** 4 / 24
    + (61 + 90 * T1 + 298 * C1 + 45 * T1 * T1 - 252 * EP2 - 3 * C1 * C1) * D ** 6 / 720);
  const lon = (D - (1 + 2 * T1 + C1) * D ** 3 / 6
    + (5 - 2 * C1 + 28 * T1 - 3 * C1 * C1 + 8 * EP2 + 24 * T1 * T1) * D ** 5 / 120) / cos;

  return { lat: lat / D2R, lon: centralMeridian(zone) + lon / D2R };
}

// 100 km square identifier (two letters) for a UTM position.
export function square100k(zone, easting, northing) {
  const col = COL_SETS[zone % 3][Math.floor(easting / 100000) - 1];
  const row = ROWS[(Math.floor(northing / 100000) + (zone % 2 === 0 ? 5 : 0)) % 20];
  return col && row ? col + row : null;
}

// Formats lat/lon as MGRS, e.g. "33U VP 12345 67890". digits is per axis
// (5 = 1 m, 4 = 10 m, 3 = 100 m ...). Returns null outside UTM coverage.
export function toMGRS(lat, lon, digits = 5) {
  const band = latBand(lat);
  if (!band) return null;
  const u = toUTM(lat, lon);
  const sq = square100k(u.zone, u.easting, u.northing);
  if (!sq) return null;
  const div = 10 ** (5 - digits);
  const e = String(Math.floor((u.easting % 100000) / div)).padStart(digits, '0');
  const n = String(Math.floor((u.northing % 100000) / div)).padStart(digits, '0');
  const zb = String(u.zone).padStart(2, '0') + band;
  return digits > 0 ? `${zb} ${sq} ${e} ${n}` : `${zb} ${sq}`;
}

// Parses an MGRS string (spaces optional) to the SW corner of the referenced
// square. Returns {lat, lon, precision} or null if it can't be parsed.
export function parseMGRS(str) {
  const s = String(str).toUpperCase().replace(/\s+/g, '');
  const m = /^(\d{1,2})([C-HJ-NP-X])([A-HJ-NP-Z])([A-HJ-NP-V])(\d{0,10})$/.exec(s);
  if (!m || m[5].length % 2) return null;
  const zone = Number(m[1]);
  if (zone < 1 || zone > 60) return null;
  const band = m[2], colL = m[3], rowL = m[4], digits = m[5];

  const col = COL_SETS[zone % 3].indexOf(colL);
  let row = ROWS.indexOf(rowL);
  if (col < 0 || row < 0) return null;
  row = (row - (zone % 2 === 0 ? 5 : 0) + 20) % 20;

  const half = digits.length / 2;
  const precision = half ? 10 ** (5 - half) : 100000;
  const e = half ? Number(digits.slice(0, half)) * precision : 0;
  const n = half ? Number(digits.slice(half)) * precision : 0;

  const easting = (col + 1) * 100000 + e;
  const bandIdx = BANDS.indexOf(band);
  const bandLat = -80 + bandIdx * 8;
  const south = bandLat < 0;
  // Row letters repeat every 2000 km; pick the cycle that lands in the band.
  const minN = toUTM(bandLat, centralMeridian(zone), zone).northing - 100000;
  let northing = row * 100000 + n;
  while (northing < minN) northing += 2000000;

  const ll = fromUTM(zone, south, easting, northing);
  if (!isFinite(ll.lat) || !isFinite(ll.lon)) return null;
  return { lat: ll.lat, lon: ll.lon, precision };
}
