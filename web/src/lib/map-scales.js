// Colour ramps and dot sizes for the live map, one place so the legend and the
// canvas cannot disagree about what a colour means.
//
// The map is read against a night ground, so the ramps are set for it: the
// stops are drawn from the together.ai product palette the site is themed on
// (deep blue #197ecf, sky #9bcdf5, lilac #ede4fc, soft orange #febea3, brand
// orange #fc4c02), lifted in lightness and chroma to hold as small marks on
// ink. Temperature diverges around a mild 15 °C, passing through a lilac-grey
// that recedes; rain climbs from that grey through cyan into a bright indigo,
// wind from slate toward white, so a gust reads as a highlight. Zero is the
// common case and should sit back rather than shout.
//
// These are the only hand-written colours outside the token block: a canvas is
// painted in JS, so it cannot read a CSS custom property per dot.
const STOPS = {
  temp: [
    [-10, [77, 143, 240]],
    [0, [143, 196, 247]],
    [15, [178, 181, 208]],
    [25, [255, 176, 131]],
    [35, [255, 95, 46]],
  ],
  rain: [
    [0, [112, 119, 148]],
    [1, [126, 206, 240]],
    [10, [72, 156, 240]],
    [30, [146, 126, 255]],
  ],
  wind: [
    [0, [104, 111, 137]],
    [5, [142, 150, 179]],
    [10, [190, 200, 226]],
    [20, [244, 249, 255]],
  ],
};

export const MODES = Object.keys(STOPS);

/** The observed value a mode paints, or null where the station does not measure it. */
export const reading = (mode, o) =>
  mode === 'temp' ? o.temp_c : mode === 'rain' ? o.precipitation_1h_mm : o.wind_ms;

function ramp(stops, v) {
  if (v <= stops[0][0]) return stops[0][1];
  for (let i = 1; i < stops.length; i++) {
    const [hi, c1] = stops[i];
    const [lo, c0] = stops[i - 1];
    if (v <= hi) {
      const f = (v - lo) / (hi - lo);
      return c0.map((c, j) => Math.round(c + (c1[j] - c) * f));
    }
  }
  return stops[stops.length - 1][1];
}

export function colour(mode, v) {
  if (v === null || v === undefined) return 'rgba(150,158,186,0.34)';
  const [r, g, b] = ramp(STOPS[mode], v);
  return `rgb(${r},${g},${b})`;
}

/**
 * A value chip's fill and a text colour that stays legible on it. The ramps run
 * from near-white to deep slate, so neither ink nor paper works for every stop:
 * the fill's own luminance decides.
 */
export function chip(mode, v) {
  const [r, g, b] = ramp(STOPS[mode], v);
  const lum = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
  return {
    fill: `rgb(${r},${g},${b})`,
    text: lum > 0.62 ? 'rgba(18,23,36,0.95)' : 'rgba(244,248,255,0.96)',
  };
}

/** The same colour as a wash, to sit a value on its own place in the scale. */
export const wash = (mode, v) =>
  v === null || v === undefined ? 'transparent' : `rgb(${ramp(STOPS[mode], v).join(',')} / 0.14)`;

// A station with nothing to report in this mode stays small and grey rather
// than disappearing: absent sensors are part of the picture.
export function radius(mode, v, zoom) {
  const base = Math.max(2.2, Math.min(6, 1.6 + zoom * 0.55));
  if (v === null || v === undefined) return base * 0.55;
  if (mode === 'rain') return base * (v > 0 ? 1 + Math.min(v, 20) / 14 : 0.7);
  if (mode === 'wind') return base * (0.75 + Math.min(v, 20) / 18);
  return base;
}

/** Eleven swatches spanning a mode's range, plus the numbers at either end. */
export function legend(mode) {
  const stops = STOPS[mode];
  const lo = stops[0][0];
  const hi = stops[stops.length - 1][0];
  const swatches = Array.from({ length: 11 }, (_, i) => colour(mode, lo + ((hi - lo) * i) / 10));
  return { lo, hi, swatches };
}
