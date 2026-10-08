import { t, pageLang } from './i18n.js';

// The API base. PUBLIC_API_BASE in .env sets it for a local run (Vite inlines
// it at build time, so it is baked into the static output, not read at
// runtime; it is a public URL, never a secret). ?api= still overrides, which
// is handy for pointing a deployed page at a branch.
// Guarded because this module is imported while React islands prerender on the
// server, where there is no location to read.
const override =
  typeof location === 'undefined' ? null : new URLSearchParams(location.search).get('api');

export const API = override || import.meta.env.PUBLIC_API_BASE || 'https://api.tenkinow.com';

/** Browser geolocation as a promise. Rejects with a message fit to display. */
export function locate() {
  return new Promise((resolve, reject) => {
    if (!navigator.geolocation) {
      reject(new Error(t(pageLang()).msg.noGeo));
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (p) => resolve({ lat: +p.coords.latitude.toFixed(4), lon: +p.coords.longitude.toFixed(4) }),
      (e) =>
        reject(
          new Error(
            e.code === e.PERMISSION_DENIED
              ? t(pageLang()).msg.blocked
              : t(pageLang()).msg.noLocation,
          ),
        ),
      { timeout: 10000, maximumAge: 300000 },
    );
  });
}

/** GET a v1 endpoint. Returns { status, ms, body }; throws only on network failure. */
export async function call(path, params) {
  const url = `${API}${path}?${new URLSearchParams(params)}`;
  const started = performance.now();
  const res = await fetch(url);
  const body = await res.json().catch(() => ({ error: 'Response was not JSON.' }));
  return { url, status: res.status, ms: Math.round(performance.now() - started), body };
}

/** A measurement, or an em dash when the station does not observe it. */
export const val = (n, digits = 1) =>
  n === null || n === undefined ? '—' : Number(n).toFixed(digits);

const POINTS = [
  'N',
  'NNE',
  'NE',
  'ENE',
  'E',
  'ESE',
  'SE',
  'SSE',
  'S',
  'SSW',
  'SW',
  'WSW',
  'W',
  'WNW',
  'NW',
  'NNW',
];
export const compass = (deg) =>
  deg === null || deg === undefined ? 'calm' : POINTS[Math.round(deg / 22.5) % 16];

/** JMA publishes JST; readings are easier to place in the time they were taken. */
export const jst = (iso) =>
  new Date(iso).toLocaleString(pageLang() === 'ja' ? 'ja-JP' : 'en-GB', {
    timeZone: 'Asia/Tokyo',
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });

/** Which of the hour's six ten-minute buckets a reading landed in. */
export const bucket = (iso) => Math.floor(new Date(iso).getUTCMinutes() / 10);
