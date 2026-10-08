// The pure logic behind the playground URL and the map's colour scale. Run with
// `pnpm test`; no browser, no build step.
import assert from 'node:assert/strict';
import test from 'node:test';
import { buildRequest, rfc } from './request.js';
import { chip, colour, legend, radius, reading } from './map-scales.js';

const BASE = 'https://api.example';

test('blank coordinates are omitted, so the API uses the network location', () => {
  const { params, url } = buildRequest({ endpoint: 'latest', lat: '  ', lon: '' }, BASE);
  assert.deepEqual(params, {});
  assert.equal(url, `${BASE}/v1/weather/latest?`);
});

test('a coordinate is trimmed and sent', () => {
  const { params } = buildRequest({ endpoint: 'latest', lat: ' 35.69 ', lon: '139.70' }, BASE);
  assert.deepEqual(params, { lat: '35.69', lon: '139.70' });
});

test('range and moment fields only travel with the endpoint that takes them', () => {
  const at = buildRequest(
    { endpoint: 'at', at: '2026-01-02T03:04', from: '2026-01-01T00:00' },
    BASE,
  );
  assert.ok(at.params.at);
  assert.equal(at.params.from, undefined);

  const history = buildRequest(
    { endpoint: 'history', from: '2026-01-01T00:00', to: '2026-01-01T06:00' },
    BASE,
  );
  assert.ok(history.params.from && history.params.to);
  assert.equal(history.params.at, undefined);
});

test('times go out as RFC3339 UTC without milliseconds', () => {
  assert.match(rfc('2026-01-02T03:04'), /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/);
  assert.equal(rfc(''), '');
});

test('the scale interpolates between stops and clamps past the ends', () => {
  assert.equal(colour('temp', -50), colour('temp', -10)); // clamped low
  assert.equal(colour('temp', 99), colour('temp', 35)); // clamped high
  assert.equal(colour('temp', 15), 'rgb(178,181,208)'); // a stop, exactly
  assert.equal(colour('temp', 20), 'rgb(217,179,170)'); // halfway to the next
});

test('a value chip takes the text colour its own fill can carry', () => {
  // The ramps run from near-white (a 20 m/s gust) to deep slate (calm), so the
  // chip's text has to follow the fill rather than the theme.
  assert.match(chip('wind', 20).text, /18,23,36/); // near-white fill, ink text
  assert.match(chip('wind', 0).text, /244,248,255/); // deep slate fill, paper text
  assert.equal(chip('temp', 35).fill, colour('temp', 35)); // same ramp as the dots
});

test('a station that does not measure the mode reads grey and small', () => {
  assert.match(colour('rain', null), /rgba/);
  assert.ok(radius('rain', null, 5) < radius('rain', 0, 5));
});

test('each mode reads its own field', () => {
  const o = { temp_c: 12, precipitation_1h_mm: 3, wind_ms: 7 };
  assert.equal(reading('temp', o), 12);
  assert.equal(reading('rain', o), 3);
  assert.equal(reading('wind', o), 7);
});

test('the legend spans the mode and matches the dots', () => {
  const { lo, hi, swatches } = legend('wind');
  assert.equal(lo, 0);
  assert.equal(hi, 20);
  assert.equal(swatches.length, 11);
  assert.equal(swatches[0], colour('wind', 0));
  assert.equal(swatches.at(-1), colour('wind', 20));
});
