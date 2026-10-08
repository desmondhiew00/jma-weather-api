/** datetime-local reads as local wall time; the API wants RFC3339 UTC. */
export const rfc = (v) => (v ? new Date(v).toISOString().replace(/\.\d{3}Z$/, 'Z') : '');

/** A Date as the `datetime-local` value for that same wall-clock moment. */
export const localValue = (d) =>
  new Date(d - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16);

/**
 * Path, query and display URL for the playground form as it stands. Empty
 * coordinate fields are left out entirely rather than sent as blanks: the API
 * reads an absent lat and lon as "use the network location".
 *
 * `base` is passed in rather than imported so this stays runnable outside a
 * browser (see smoke.test.js).
 */
export function buildRequest({ endpoint, lat = '', lon = '', from = '', to = '', at = '' }, base) {
  const params = {};
  if (lat.trim()) params.lat = lat.trim();
  if (lon.trim()) params.lon = lon.trim();
  if (endpoint === 'history') {
    params.from = rfc(from);
    params.to = rfc(to);
  }
  if (endpoint === 'at') params.at = rfc(at);

  const path = `/v1/weather/${endpoint}`;
  return { path, params, url: `${base}${path}?${new URLSearchParams(params)}` };
}
