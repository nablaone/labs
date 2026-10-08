// Generates internal/geo/testdata/vectors.json from the browser code, so the
// Go port (internal/geo) is checked against exactly what the client does.
// Run from sitaw/: node web/tools/gen-geo-vectors.mjs
import { writeFileSync } from 'node:fs';
import { toMGRS, parseMGRS } from '../static/js/mgrs.js';
import { parseAnyCoord } from '../static/js/coords.js';

let seed = 42; // deterministic
const rnd = () => ((seed = (seed * 1103515245 + 12345) % 2147483648) / 2147483648);

const toMgrs = [];
for (let i = 0; i < 500; i++) {
  const lat = -79.99 + rnd() * 163.98, lon = -180 + rnd() * 359.999;
  for (const digits of [5, 3]) toMgrs.push({ lat, lon, digits, mgrs: toMGRS(lat, lon, digits) });
}
const fromMgrs = toMgrs.filter((v) => v.mgrs).slice(0, 400).map(({ mgrs }) => ({ mgrs, ...parseMGRS(mgrs) }));
const inputs = [
  '34U DC 12345 67890', '34UDC1234', '34udc', '52.2297, 21.0122', '52.2297 21.0122', '52.2297;21.0122',
  '-33.8568, 151.2153', '-33.8568 151.2153', '52.2297N 21.0122E', 'N52.2297 E21.0122', '21.0122E 52.2297N',
  '33.8568 S, 151.2153 E', '40.7128N, 74.0060W', `52°13'47"N 21°0'44"E`, '52°13′47″N, 21°00′44″E',
  '52 13 47 N 21 0 44 E', '52 13 47 21 0 44', '52 13.7833 N 21 0.7333 E', 'N 52 13 47 E 21 0 44',
  'WP-1', 'recon', 'RV', '95, 10', '52, 200', '52 13 61 N 21 0 0 E', '52.5', '-52 S, 21 E', '52N 21N', '', '1 2 3',
  '52,2297, 21,0122', '+52.1 +21.2', '52.1N, 21.2W',
];
const parse = inputs.map((input) => ({ input, result: parseAnyCoord(input) }));
writeFileSync(new URL('../../internal/geo/testdata/vectors.json', import.meta.url), JSON.stringify({ toMgrs, fromMgrs, parse }));
console.log(`wrote ${toMgrs.length} toMgrs, ${fromMgrs.length} fromMgrs, ${parse.length} parse vectors`);
