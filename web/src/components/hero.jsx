import { useEffect, useRef, useState } from 'react';
import { API, call, locate, val, compass, jst, bucket } from '../lib/wx.js';
import { t } from '../lib/i18n.js';

// Tokyo. The coordinate the form opens on, and the reading the page opens on.
const SEED = '35.69, 139.70';

/**
 * Counts a figure up from zero the first time a real reading lands, then tracks
 * the value directly. Only the first arrival is worth animating: a ten-minute
 * refresh that re-ticks the whole number reads as a glitch.
 * Reduced-motion readers get the value immediately.
 */
function useCountUp(target, active) {
  const [shown, setShown] = useState(null);
  const ticked = useRef(false);

  useEffect(() => {
    if (target === null || target === undefined) {
      setShown(null);
      return;
    }
    // A refresh is in flight: hold the last reading. The panel dims and the
    // control spins to say a request is running; blanking the figure to a dash
    // would say "no data" instead of "one moment".
    if (!active) return;
    const calm =
      typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (ticked.current || calm) {
      setShown(target);
      return;
    }
    ticked.current = true;

    let frame;
    const started = performance.now();
    const step = (now) => {
      const p = Math.min(1, (now - started) / 500);
      // The same ease-out the CSS uses, approximated: fast out, settles late.
      setShown(target * (1 - (1 - p) ** 3));
      if (p < 1) frame = requestAnimationFrame(step);
    };
    frame = requestAnimationFrame(step);
    return () => cancelAnimationFrame(frame);
  }, [target, active]);

  return shown;
}

/** One measurement in the spec strip: label, value, unit. */
function Spec({ label, value, unit, tone = 'text-ink' }) {
  return (
    <div>
      <dt className="label">{label}</dt>
      <dd
        className={`mono mt-1 mb-0 text-[1.4rem] leading-none font-medium ${
          value === '—' ? 'text-rule-2' : tone
        }`}
      >
        {value}
        <i className="unit">{unit}</i>
      </dd>
    </div>
  );
}

/**
 * The landing hero. Interactive, so it is a React island: the locate button,
 * the coordinate form and the readout all read from one piece of state.
 *
 * Stat-Led: the live temperature is the largest thing on the page, it is the
 * first thing on the page, and the words beside it say whose temperature it is.
 *
 * Only `lang` crosses from Astro. The strings carry functions, which would not
 * survive being serialized onto the island.
 */
export default function Hero({ lang = 'en' }) {
  const s = t(lang).home;
  const msg = t(lang).msg;

  const [coord, setCoord] = useState(SEED);
  const [state, setState] = useState('idle'); // idle | loading | ok
  const [reading, setReading] = useState(null); // { station, observation }
  const [status, setStatus] = useState('');
  const [bad, setBad] = useState(false); // the coordinate field failed to parse
  const [locating, setLocating] = useState(false);

  // A precise reading always wins, whenever it lands. Without this the slower
  // of the two requests would overwrite the better answer.
  const precise = useRef(false);

  // The figure is the first and largest thing on the page, so it opens holding a
  // real measurement rather than a dash. It reads the coordinate the form is
  // already showing, which needs no permission prompt and no interaction;
  // locating, or typing a coordinate, replaces it. It skips the coordinate-less
  // call because /v1/weather/latest answers that with `lat is required`, and a
  // reader would open the page on an error instead of a reading.
  useEffect(() => {
    const [lat, lon] = SEED.split(', ').map(Number);
    show(lat, lon);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // No coordinate means the API answers from the location its edge derives from
  // the request IP, which is how a reader who will not (or cannot) share a
  // location still gets a reading.
  async function show(lat, lon) {
    const fromNetwork = lat === undefined;
    if (fromNetwork && precise.current) return;

    setState('loading');
    setStatus('');

    // Only the request itself is allowed to report "unreachable". Wrapping the
    // whole success path in one catch meant any mistake reading the response
    // (a missing field, say) was announced as a network failure against a host
    // the page had never contacted.
    let res;
    try {
      res = await call('/v1/weather/latest', fromNetwork ? {} : { lat, lon });
    } catch {
      setState('idle');
      setStatus(msg.unreachable(new URL(API).host));
      return;
    }

    const { status: code, body } = res;
    if (code !== 200) {
      setState('idle');
      setStatus(code === 404 ? msg.outside : body.error || msg.apiSaid(code));
      return;
    }
    if (fromNetwork && precise.current) return;
    precise.current = precise.current || !fromNetwork;

    setReading(body);
    setState('ok');

    // `coordinate` says which point the answer was computed from, and whether it
    // came from the query or from the edge's IP lookup. API builds before that
    // field existed omit it, so fall back to the station's own position rather
    // than throwing on a response that is otherwise perfectly good.
    const from = body.coordinate;
    setCoord(from ? `${from.lat}, ${from.lon}` : `${body.station.lat}, ${body.station.lon}`);
    setStatus(from?.source === 'network' ? msg.fromNetwork : '');
  }

  async function onLocate() {
    setLocating(true);
    setStatus('');

    // Both at once: the network location fills the readout straight away, and
    // the precise one replaces it if the reader allows it. Waiting on a prompt
    // nobody answers would otherwise leave the page blank for ten seconds.
    show();
    try {
      const { lat, lon } = await locate();
      await show(lat, lon);
    } catch {
      // Denied or ignored; the network reading already stands.
    } finally {
      setLocating(false);
    }
  }

  function onSubmit(e) {
    e.preventDefault();
    const [lat, lon] = coord.split(/[,\s]+/).map(Number);
    if (!isFinite(lat) || !isFinite(lon)) {
      setBad(true);
      setStatus(msg.badCoord);
      return;
    }
    setBad(false);
    precise.current = true;
    show(lat, lon);
  }

  const o = reading?.observation;
  const st = reading?.station;
  const lit = o ? bucket(o.observed_at) : -1;
  const ticked = useCountUp(o?.temp_c, state === 'ok');
  const busy = state === 'loading';

  return (
    <>
      {/* The reading comes first, and the two ways to make it yours sit beside
          it. A reader who came for the weather should not have to get through a
          pitch to find it. The pitch is below, for the reader who stays.

          The station name is set as a value, not a heading: the page's first
          heading has to be its h1, which is in the section underneath. */}
      <section
        className="wrap pt-[clamp(1rem,3vw,1.75rem)]"
        aria-label={s.nowLabel}
        aria-busy={busy || undefined}
      >
        <div
          className={`panel px-6 py-7 transition-opacity duration-200 ease-out sm:px-9 sm:py-9 ${
            busy ? 'opacity-60' : 'opacity-100'
          }`}
        >
          <div className="grid gap-x-12 gap-y-9 lg:grid-cols-[minmax(0,1fr)_minmax(0,21rem)] lg:items-start">
            <div>
              <p className="label m-0">{s.nowLabel}</p>

              <div className="mt-2 flex flex-wrap items-end gap-x-9 gap-y-4">
                <div
                  className={`font-display tnum text-figure leading-[0.85] font-extrabold tracking-[-0.045em] whitespace-nowrap ${
                    o ? 'text-ink' : 'text-rule-2'
                  }`}
                >
                  {ticked === null ? '—' : ticked.toFixed(1)}
                  <i className="unit text-[0.22em] font-semibold tracking-normal">°C</i>
                </div>

                <div className="pb-2">
                  <div className="font-display text-[clamp(1.35rem,3vw,1.85rem)] leading-tight font-bold tracking-[-0.03em] text-ink">
                    {st ? st.name : s.stationIdle}
                  </div>
                  <p className="mono mt-1 mb-0 text-[0.8rem] text-muted">
                    {st
                      ? `${st.name_en} · ${msg.away(st.distance_km, st.altitude_m)}`
                      : s.stationIdleEn}
                  </p>

                  <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2">
                    {/* Six ticks: JMA publishes on ten-minute boundaries, so this
                        says which slice of the hour the reading belongs to. */}
                    <div className="flex items-end gap-[5px]" aria-hidden="true">
                      {[0, 1, 2, 3, 4, 5].map((i) => (
                        <span
                          key={i}
                          className={`w-0.5 transition-all duration-200 ease-out ${
                            i === lit ? 'h-4 bg-accent' : 'h-2 bg-rule-2'
                          }`}
                        />
                      ))}
                    </div>
                    <span className="mono text-[0.8rem] text-muted">
                      {o ? `${jst(o.observed_at)} JST` : s.bucketIdle}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            {/* The controls belong with the reading they change. The rule only
                appears once the pair is side by side. */}
            <div className="lg:border-l lg:border-rule lg:pl-11">
              <button
                className="btn w-full"
                type="button"
                onClick={onLocate}
                disabled={locating}
                data-state={locating ? 'loading' : undefined}
              >
                {locating && <span className="spinner" aria-hidden="true" />}
                {s.locate}
              </button>

              <form className="mt-6" onSubmit={onSubmit}>
                <label htmlFor="coord">{s.orCoord}</label>
                <div className="flex gap-2">
                  <input
                    id="coord"
                    name="coord"
                    className="field min-w-0"
                    value={coord}
                    onChange={(e) => {
                      setCoord(e.target.value);
                      setBad(false);
                    }}
                    inputMode="decimal"
                    aria-invalid={bad || undefined}
                    aria-describedby="coord-help"
                  />
                  <button
                    className="chip gap-2 px-4"
                    type="submit"
                    disabled={busy && !locating}
                    data-state={busy && !locating ? 'loading' : undefined}
                  >
                    {busy && !locating && <span className="spinner" aria-hidden="true" />}
                    {s.read}
                  </button>
                </div>
                <p id="coord-help" className="mt-2 mb-0 text-[0.78rem] text-muted">
                  {s.coordHelp}
                </p>
              </form>
            </div>
          </div>

          <p
            className={`mono mb-0 text-[0.78rem] text-warn ${status ? 'mt-7' : 'mt-0'}`}
            role="status"
          >
            {status}
          </p>

          {/* Every field the station reports, on one rule grid. */}
          <dl className="mt-9 grid grid-cols-2 gap-x-6 gap-y-7 border-t border-rule pt-7 sm:grid-cols-3 lg:grid-cols-6">
            <Spec label={s.fields.humidity} value={o ? val(o.humidity_pct, 0) : '—'} unit="%" />
            <Spec label={s.fields.pressure} value={o ? val(o.pressure_hpa) : '—'} unit="hPa" />
            <Spec
              label={s.fields.wind}
              value={o ? val(o.wind_ms) : '—'}
              unit={o ? `m/s ${compass(o.wind_direction_deg)}` : 'm/s'}
            />
            <Spec label={s.fields.rain} value={o ? val(o.precipitation_1h_mm) : '—'} unit="mm" />
            <Spec label={s.fields.sun} value={o ? val(o.sun_1h_h) : '—'} unit="h" />
            <Spec label={s.fields.snow} value={o ? val(o.snow_cm, 0) : '—'} unit="cm" />
          </dl>

          <p className="mt-8 mb-0 text-[0.85rem] text-muted">
            {s.note} <code className="text-ink">null</code>.
          </p>
        </div>
      </section>

      {/* What the reading above is, for the reader who stays. */}
      <section className="wrap mt-[clamp(3.5rem,8vw,6rem)]">
        <div className="grid gap-x-14 gap-y-6 lg:grid-cols-[minmax(0,1.05fr)_minmax(0,0.95fr)] lg:items-start">
          <h1 className="max-w-[22ch]">
            {s.h1[0]}
            <br />
            <span className="h-quiet">{s.h1[1]}</span>
          </h1>
          <p className="mb-0 text-neutral lg:pt-2">{s.lede}</p>
        </div>
      </section>
    </>
  );
}
