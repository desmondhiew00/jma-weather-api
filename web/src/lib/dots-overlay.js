import { chip, colour, radius, reading } from './map-scales.js';

// Close in, there is room for the reading itself, so a station stops being a
// colour to interpret and becomes a number to read. Zoom 9 is about one
// prefecture across, the first view where the chips do not collide everywhere.
export const CHIP_ZOOM = 9;
// Bold and a size up: a chip has to beat the basemap's own place names, which
// sit at a similar size all around it, and be readable at a glance from a metre
// back rather than only when leaned into.
const CHIP_FONT = '600 13px "Geist Mono", ui-monospace, SFMono-Regular, monospace';
const CHIP_H = 21;
// A dark edge separates a pale chip from a pale place name behind it. It used
// to be a canvas shadow as well, which is the most expensive thing a 2d context
// does; a few hundred of them per frame made a drag feel heavy. A
// hairline costs nothing and does the same job on a night map.
const CHIP_EDGE = 'rgba(8,11,20,0.7)';

// Chip widths, by the text in them. measureText is not free and `draw()` runs
// on every frame of a pan, while the set of distinct readings is tiny.
const widths = new Map();

// One decimal, as the API reports it and the card shows it: people read a
// station map for 20.7, not 21. Only a three-digit reading
// drops it, and none of the three modes gets there in practice.
const fmt = (v) => (Math.abs(v) < 100 ? v.toFixed(1) : String(Math.round(v)));

const hits = (a, b) => a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;

function chipPath(ctx, { x, y, w, h }) {
  ctx.beginPath();
  ctx.roundRect(x, y, w, h, 5);
}

// The selection ring. Neutral on purpose: the dots use hue to encode a
// measurement, so a ring in the accent orange would read as a hot reading
// rather than as "this is the one you are looking at". The pale ring is laid
// down first and slightly thicker, so the dark one reads at any basemap
// lightness. Both mirror tokens in global.css, because a canvas cannot read a
// custom property.
const RING_HALO = 'rgba(253, 253, 254, 0.92)';
const RING_INK = 'rgba(27, 32, 48, 0.85)';

/**
 * One canvas for ~1,300 dots: a marker per station would cost 1,300 DOM nodes
 * and stutter on every pan.
 *
 * `OverlayView` is passed in rather than imported, because it only exists once
 * the Maps JS library has loaded. `getState()` supplies the current stations
 * and mode, so a redraw always paints what the page is showing now; the dots it
 * painted are left on `overlay.painted` for hit testing.
 */
export function createDotsOverlay(maps, getState) {
  class Dots extends maps.OverlayView {
    painted = []; // {x, y, st} in canvas pixels, refreshed on every draw

    onAdd() {
      this.canvas = document.createElement('canvas');
      this.canvas.style.position = 'absolute';
      this.canvas.style.pointerEvents = 'none';
      this.getPanes().overlayLayer.appendChild(this.canvas);
    }

    onRemove() {
      this.canvas.remove();
    }

    draw() {
      const proj = this.getProjection();
      if (!proj) return;

      const bounds = this.map.getBounds();
      if (!bounds) return;

      const { stations, mode, active } = getState();

      const sw = proj.fromLatLngToDivPixel(bounds.getSouthWest());
      const ne = proj.fromLatLngToDivPixel(bounds.getNorthEast());
      const pad = 40;
      const left = Math.min(sw.x, ne.x) - pad;
      const top = Math.min(sw.y, ne.y) - pad;
      const w = Math.abs(ne.x - sw.x) + pad * 2;
      const h = Math.abs(sw.y - ne.y) + pad * 2;
      const dpr = window.devicePixelRatio || 1;

      Object.assign(this.canvas.style, {
        left: `${left}px`,
        top: `${top}px`,
        width: `${w}px`,
        height: `${h}px`,
      });
      this.canvas.width = w * dpr;
      this.canvas.height = h * dpr;

      const ctx = this.canvas.getContext('2d');
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, w, h);

      const zoom = this.map.getZoom();
      this.painted = [];

      // Held back and drawn after the loop: a dot painted later would otherwise
      // cross the ring of a station that sits behind it.
      let selected = null;

      // Chips are placed first-come and skipped where one would cover another,
      // so a dense city reads as a few numbers over dots rather than as a pile
      // of overlapping boxes. Whatever loses the place stays a dot.
      const chips = zoom >= CHIP_ZOOM;
      const placed = [];
      if (chips) {
        ctx.font = CHIP_FONT;
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
      }

      // Reject by coordinate before projecting: a LatLng and a projection for
      // all ~1,300 stations on every frame of a pan is most of the work, and at
      // a chip zoom fewer than a tenth of them are on screen.
      const view = bounds.toJSON();
      const slack = 0.5; // degrees, so a dot just off the edge still paints

      for (const st of stations) {
        if (
          st.lat < view.south - slack ||
          st.lat > view.north + slack ||
          st.lon < view.west - slack ||
          st.lon > view.east + slack
        ) {
          continue;
        }

        const p = proj.fromLatLngToDivPixel(new maps.LatLng(st.lat, st.lon));
        const x = p.x - left;
        const y = p.y - top;
        if (x < -10 || y < -10 || x > w + 10 || y > h + 10) continue;

        const v = reading(mode, st.observation);

        if (
          mode === 'wind' &&
          st.observation.wind_direction_deg !== null &&
          st.observation.wind_ms
        ) {
          // A stem pointing the way the wind is going, so direction is readable
          // at a glance and not only in the card.
          const rad = ((st.observation.wind_direction_deg + 180) % 360) * (Math.PI / 180);
          const len = 4 + Math.min(st.observation.wind_ms, 20) * 0.7;
          ctx.beginPath();
          ctx.moveTo(x, y);
          ctx.lineTo(x + Math.sin(rad) * len, y - Math.cos(rad) * len);
          ctx.strokeStyle = colour(mode, v);
          ctx.lineWidth = 1.2;
          ctx.stroke();
        }

        // A dry station is the common case, and a thousand chips reading 0.0
        // would bury the handful that are actually raining: zero keeps its
        // small recessive dot.
        let box = null;
        if (chips && v !== null && v !== undefined && !(mode === 'rain' && v === 0)) {
          const text = fmt(v);
          let cw = widths.get(text);
          if (cw === undefined) {
            cw = Math.round(ctx.measureText(text).width) + 14;
            widths.set(text, cw);
          }
          const candidate = { x: x - cw / 2, y: y - CHIP_H / 2, w: cw, h: CHIP_H };
          if (!placed.some((b) => hits(b, candidate))) {
            const { fill, text: onFill } = chip(mode, v);
            placed.push(candidate);
            box = candidate;
            chipPath(ctx, box);
            ctx.fillStyle = fill;
            ctx.fill();
            ctx.strokeStyle = CHIP_EDGE;
            ctx.lineWidth = 1;
            ctx.stroke();
            ctx.fillStyle = onFill;
            ctx.fillText(text, x, y + 0.5);
          }
        }

        if (!box) {
          ctx.beginPath();
          ctx.arc(x, y, radius(mode, v, zoom), 0, Math.PI * 2);
          ctx.fillStyle = colour(mode, v);
          ctx.fill();
        }

        this.painted.push({ x, y, st });
        if (active && st.id === active) selected = { x, y, r: radius(mode, v, zoom), box };
      }

      if (selected) {
        for (const [stroke, width] of [
          [RING_HALO, 4],
          [RING_INK, 1.5],
        ]) {
          if (selected.box) {
            const pad = width / 2 + 2;
            const b = selected.box;
            chipPath(ctx, {
              x: b.x - pad,
              y: b.y - pad,
              w: b.w + pad * 2,
              h: b.h + pad * 2,
            });
          } else {
            ctx.beginPath();
            ctx.arc(selected.x, selected.y, selected.r + 5, 0, Math.PI * 2);
          }
          ctx.strokeStyle = stroke;
          ctx.lineWidth = width;
          ctx.stroke();
        }
      }
    }

    /**
     * The nearest painted dot to a pointer event, so a 3-pixel target is not
     * what the reader has to hit. A finger needs a bigger reach than a mouse.
     */
    pick(e) {
      if (!this.canvas || !this.painted.length) return null;
      const r = this.canvas.getBoundingClientRect();
      const px = e.clientX - r.left;
      const py = e.clientY - r.top;

      let best = null;
      let bestD = Infinity;
      for (const p of this.painted) {
        const d = (p.x - px) ** 2 + (p.y - py) ** 2;
        if (d < bestD) {
          bestD = d;
          best = p;
        }
      }

      const reach = e.pointerType === 'mouse' ? 40 : 64;
      return best && bestD < reach ** 2 ? best.st : null;
    }
  }

  return new Dots();
}

/**
 * A night basemap. The live map is the one dark room on the site: land is a
 * cool near-navy, water a step deeper still so the sea recedes, and the only
 * lines left are prefecture boundaries, so nothing on the base competes with
 * a station dot. On ink the dots' colour reads at full strength.
 */
export const BASEMAP_STYLES = [
  { elementType: 'geometry', stylers: [{ color: '#1b2132' }] },
  { elementType: 'labels.text.fill', stylers: [{ color: '#8b94ad' }] },
  { elementType: 'labels.text.stroke', stylers: [{ color: '#141a29' }] },
  { featureType: 'administrative', elementType: 'geometry', stylers: [{ color: '#2b3346' }] },
  { featureType: 'administrative.land_parcel', stylers: [{ visibility: 'off' }] },
  { featureType: 'poi', stylers: [{ visibility: 'off' }] },
  { featureType: 'road', stylers: [{ visibility: 'off' }] },
  { featureType: 'transit', stylers: [{ visibility: 'off' }] },
  { featureType: 'landscape', elementType: 'geometry', stylers: [{ color: '#1e2536' }] },
  { featureType: 'water', elementType: 'geometry', stylers: [{ color: '#121724' }] },
  { featureType: 'water', elementType: 'labels', stylers: [{ visibility: 'off' }] },
];

/**
 * The same basemap with its own place names taken down a step. Once the chips
 * are on, Google's labels and the readings are the same size in the same place,
 * so the names step back: they stay legible for orientation and the numbers
 * get the foreground.
 */
export const QUIET_STYLES = BASEMAP_STYLES.map((rule) =>
  rule.elementType === 'labels.text.fill' ? { ...rule, stylers: [{ color: '#646c82' }] } : rule,
);
