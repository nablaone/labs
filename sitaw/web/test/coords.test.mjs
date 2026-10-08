import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseAnyCoord } from '../static/js/coords.js';

const W = [52.229722, 21.012222]; // 52°13'47"N 21°0'44"E (Warsaw)

const parses = [
  ['52.2297, 21.0122', [52.2297, 21.0122]],
  ['52.2297 21.0122', [52.2297, 21.0122]],
  ['52.2297;21.0122', [52.2297, 21.0122]],
  ['-33.8568, 151.2153', [-33.8568, 151.2153]],
  ['-33.8568 151.2153', [-33.8568, 151.2153]],
  ['52.2297N 21.0122E', [52.2297, 21.0122]],
  ['N52.2297 E21.0122', [52.2297, 21.0122]],
  ['21.0122E 52.2297N', [52.2297, 21.0122]],
  ['33.8568 S, 151.2153 E', [-33.8568, 151.2153]],
  ['40.7128N, 74.0060W', [40.7128, -74.006]],
  [`52°13'47"N 21°0'44"E`, W],
  ['52°13′47″N, 21°00′44″E', W],
  ['52 13 47 N 21 0 44 E', W],
  ['52 13 47 21 0 44', W],
  ['52 13.7833 N 21 0.7333 E', W],
  ['N 52 13 47 E 21 0 44', W],
];

for (const [input, [lat, lon]] of parses) {
  test(`degrees: ${input}`, () => {
    const p = parseAnyCoord(input);
    assert.ok(p, 'should parse');
    assert.equal(p.kind, 'deg');
    assert.ok(Math.abs(p.lat - lat) < 1e-3 && Math.abs(p.lon - lon) < 1e-3, `got ${p.lat}, ${p.lon}`);
  });
}

for (const input of ['34U DC 12345 67890', '34UDC1234', '34udc']) {
  test(`mgrs: ${input}`, () => assert.equal(parseAnyCoord(input)?.kind, 'mgrs'));
}

// Search text must not be mistaken for a position.
for (const input of ['WP-1', 'recon', 'RV', '95, 10', '52, 200', '52 13 61 N 21 0 0 E', '52.5',
  '-52 S, 21 E', '52N 21N', '', '1 2 3']) {
  test(`not a coordinate: ${JSON.stringify(input)}`, () => assert.equal(parseAnyCoord(input), null));
}
