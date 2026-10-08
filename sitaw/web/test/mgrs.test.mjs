import { test } from 'node:test';
import assert from 'node:assert/strict';
import { toMGRS, parseMGRS } from '../static/js/mgrs.js';

test('MGRS round trip stays within the 1 m square', () => {
  let worst = 0;
  for (let i = 0; i < 20000; i++) {
    const lat = -79.9 + Math.random() * 163.8, lon = -180 + Math.random() * 359.99;
    const p = parseMGRS(toMGRS(lat, lon));
    assert.ok(p, `parse ${lat},${lon}`);
    const dy = (p.lat - lat) * 111320, dx = (p.lon - lon) * 111320 * Math.cos(lat * Math.PI / 180);
    worst = Math.max(worst, Math.hypot(dx, dy));
  }
  assert.ok(worst < 2, `worst error ${worst} m`);
});

test('known zone and square', () => {
  assert.match(toMGRS(38.8895, -77.0353), /^18S UJ /);
  assert.match(toMGRS(52.2297, 21.0122), /^34U /);
});
