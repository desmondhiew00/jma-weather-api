// Regenerates src/data/stations.json from the live API, so building the station
// pages needs no network and a JMA outage can never fail a deploy. The station
// list changes a few times a year at most; rerun this when it does.
//
//   node scripts/stations.mjs
import { writeFileSync } from 'node:fs';

const SRC = 'https://api.tenkinow.com/v1/weather/now';

const slugify = (s) =>
  s
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '');

const res = await fetch(SRC);
if (!res.ok) throw new Error(`${SRC} answered ${res.status}`);
const { stations } = await res.json();

// name_en is not unique across the network (two 大野, two Kawanishi…), so a
// colliding slug takes the station id as a suffix. Sorting by id first keeps the
// suffix assignment stable between runs.
const seen = new Map();
const rows = stations
  .sort((a, b) => a.id.localeCompare(b.id))
  .map(({ id, name, name_en, lat, lon, altitude_m }) => {
    const base = slugify(name_en) || id;
    const n = (seen.get(base) ?? 0) + 1;
    seen.set(base, n);
    return {
      id,
      slug: n === 1 ? base : `${base}-${id}`,
      name,
      name_en,
      lat,
      lon,
      altitude_m,
    };
  });

// The three nearest stations, precomputed. They make each station page differ
// from its neighbours, and they give the crawler a path from any
// station into the rest of the network.
const R = 6371;
const rad = (d) => (d * Math.PI) / 180;
const km = (a, b) => {
  const dLat = rad(b.lat - a.lat);
  const dLon = rad(b.lon - a.lon);
  const h =
    Math.sin(dLat / 2) ** 2 + Math.cos(rad(a.lat)) * Math.cos(rad(b.lat)) * Math.sin(dLon / 2) ** 2;
  return 2 * R * Math.asin(Math.sqrt(h));
};
for (const s of rows) {
  s.near = rows
    .filter((o) => o.id !== s.id)
    .map((o) => ({ slug: o.slug, name: o.name, name_en: o.name_en, km: +km(s, o).toFixed(1) }))
    .sort((a, b) => a.km - b.km)
    .slice(0, 3);
}

writeFileSync('src/data/stations.json', JSON.stringify(rows) + '\n');
console.log(`${rows.length} stations → src/data/stations.json`);
