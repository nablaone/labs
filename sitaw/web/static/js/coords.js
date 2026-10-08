// Coordinate formatting/parsing used by the whole UI. Display is MGRS for now;
// other display formats (DD, DMS, UTM) plug in here and get picked from settings.
// Input accepts MGRS and lat/lon in degrees (see parseAnyCoord).

import { toMGRS, parseMGRS } from './mgrs.js';

export function formatCoord(lat, lon) {
  return toMGRS(lat, lon, 5) ?? formatDegrees(lat, lon);
}

export function formatDegrees(lat, lon) {
  return `${lat.toFixed(5)}, ${lon.toFixed(5)}`;
}

export function formatDistance(m) {
  return m < 1000 ? `${Math.round(m)} m` : `${(m / 1000).toFixed(m < 10000 ? 2 : 1)} km`;
}

// Parses anything a person might type as a position:
//   MGRS            34U DC 12345 67890, 34UDC1234, 34UDC
//   decimal         52.2297, 21.0122 · 52.2297 21.0122 · -33.86 151.21
//   with hemisphere 52.2297N 21.0122E · N52.2297 E21.0122 · 33.86 S, 151.21 E
//   deg-min(-sec)   52°13'47"N 21°0'44"E · 52 13.78 N 21 0.73 E · 52°13′47″ 21°00′44″
// Without hemisphere letters the first value is latitude. Returns
// {lat, lon, precision (m, approximate), kind: 'mgrs'|'deg'} or null.
export function parseAnyCoord(input) {
  const s = String(input).trim();
  if (!s) return null;
  const m = parseMGRS(s);
  if (m) return { ...m, kind: 'mgrs' };
  const d = parseDegrees(s);
  return d ? { ...d, kind: 'deg' } : null;
}

const HEMI = /[NSEW]/;

function parseDegrees(input) {
  // Normalize symbols to spaces but keep separators and hemisphere letters.
  let s = input.toUpperCase()
    .replace(/[°º˚]|DEG/g, ' ').replace(/["″”]|''/g, ' ').replace(/['′’]/g, ' ');
  if (/[^0-9NSEW.,;\s+-]/.test(s)) return null;

  let parts;
  if (/[,;]/.test(s)) {
    parts = s.split(/[,;]/);
  } else {
    // Split after the first hemisphere letter that ends a value ("52 13 N 21 0 E"),
    // or before one that starts the second value ("N52.1 E21.0").
    const toks = s.trim().split(/\s+|(?<=[NSEW])(?=[\d+-])|(?<=\d)(?=[NSEW])/).filter(Boolean);
    const lead = HEMI.test(toks[0]);
    let cut = -1;
    if (lead) cut = toks.findIndex((t, i) => i > 0 && HEMI.test(t));
    else {
      const i = toks.findIndex((t) => HEMI.test(t));
      if (i >= 0 && i < toks.length - 1) cut = i + 1;
    }
    if (cut < 0) {
      // No letters: split the numbers evenly (2 → d d, 4 → dm dm, 6 → dms dms).
      if (toks.some((t) => HEMI.test(t)) || toks.length % 2 || toks.length > 6) return null;
      cut = toks.length / 2;
    }
    parts = [toks.slice(0, cut).join(' '), toks.slice(cut).join(' ')];
  }
  if (parts.length !== 2) return null;
  const a = parseAngle(parts[0]), b = parseAngle(parts[1]);
  if (!a || !b) return null;

  // Hemisphere letters decide which value is which; otherwise lat comes first.
  const isLat = (h) => h === 'N' || h === 'S';
  const isLon = (h) => h === 'E' || h === 'W';
  let lat = a, lon = b;
  if (isLon(a.hemi) || isLat(b.hemi)) [lat, lon] = [b, a];
  if ((lat.hemi && !isLat(lat.hemi)) || (lon.hemi && !isLon(lon.hemi))) return null;
  if (Math.abs(lat.v) > 90 || Math.abs(lon.v) > 180) return null;
  return { lat: lat.v, lon: lon.v, precision: Math.min(a.precision, b.precision) };
}

// One angle: "52.2297", "-33.86", "52 13 47 N", "N 52 13.78", "+21.0".
function parseAngle(part) {
  const toks = part.trim().split(/\s+|(?<=[NSEW])(?=[\d+-])|(?<=\d)(?=[NSEW])/).filter(Boolean);
  const letters = toks.filter((t) => HEMI.test(t));
  const nums = toks.filter((t) => !HEMI.test(t));
  if (letters.length > 1 || letters.some((l) => l.length !== 1) || nums.length < 1 || nums.length > 3) return null;
  if (!nums.every((n, i) => (i === 0 ? /^[+-]?\d+(\.\d+)?$/ : /^\d+(\.\d+)?$/).test(n))) return null;
  // Only the last component may have decimals; minutes/seconds stay below 60.
  if (nums.slice(0, -1).some((n) => n.includes('.'))) return null;
  const [d, m = '0', sec = '0'] = nums;
  if (Number(m) >= 60 || Number(sec) >= 60) return null;
  const neg = d.startsWith('-');
  let v = Math.abs(Number(d)) + Number(m) / 60 + Number(sec) / 3600;
  const hemi = letters[0] ?? '';
  if (neg && hemi) return null; // "-52 S" is ambiguous
  if (neg || hemi === 'S' || hemi === 'W') v = -v;
  // Rough ground size of the last digit, for choosing a zoom level.
  const decimals = (nums[nums.length - 1].split('.')[1] ?? '').length;
  const unitM = [111000, 1850, 31][nums.length - 1];
  return { v, hemi, precision: Math.max(1, unitM / 10 ** decimals) };
}

// Kept for callers that only want MGRS.
export function parseCoord(s) {
  return parseMGRS(s);
}
