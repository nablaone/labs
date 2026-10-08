import { test } from 'node:test';
import assert from 'node:assert/strict';
import { formatArea, bearingTrue, bearingGrid, formatBearing, formatDistance } from '../static/js/coords.js';

test('formatArea picks m², ha or km²', () => {
  assert.equal(formatArea(850), '850 m²');
  assert.equal(formatArea(12345), '1.23 ha');
  assert.equal(formatArea(456000), '45.6 ha');
  assert.equal(formatArea(4554020), '4.55 km²');
  assert.equal(formatArea(25e6), '25.0 km²');
});

test('bearings', () => {
  assert.ok(Math.abs(bearingTrue(0, 0, 1, 0)) < 1e-9, 'due north');
  assert.ok(Math.abs(bearingTrue(0, 0, 0, 1) - 90) < 1e-9, 'due east');
  // Warsaw -> Kraków ~196.5° true, as checked server-side.
  assert.ok(Math.abs(bearingTrue(52.2318, 21.0060, 50.0617, 19.9373) - 196.5) < 2);
  // On the zone 34 central meridian (21°E) grid = true; east of it grid < true.
  assert.ok(Math.abs(bearingGrid(52, 21, 52.1, 21) - bearingTrue(52, 21, 52.1, 21)) < 1e-9);
  assert.ok(bearingGrid(52, 23, 52.1, 23) > 358, 'north east of the CM reads just under 360 grid');
  assert.equal(formatBearing(45.4), '045°');
  assert.equal(formatBearing(359.7), '000°');
  assert.equal(formatDistance(1240), '1.24 km');
});
