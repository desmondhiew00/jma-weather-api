import { useEffect, useState } from 'react';
import { call, val, compass, jst } from '../lib/wx.js';
import { t } from '../lib/i18n.js';

/**
 * The current reading for one station, fetched on load.
 *
 * Deliberately not prerendered: a station page is rebuilt only when the site is
 * deployed, so a temperature baked into the HTML would be weeks old and would be
 * the one number on the page a reader trusts most. Search comes for the static
 * half of the page (where the station is, its id, how to call it); this live
 * reading is for the human who arrived.
 *
 * Only `lang` and the coordinate cross from Astro: the strings carry functions,
 * which would not survive serialisation into the island's props.
 */
export default function StationReading({ lang = 'en', lat, lon }) {
  const s = t(lang).station;
  const fields = t(lang).home.fields;
  const [state, setState] = useState({ status: 'loading' });

  useEffect(() => {
    let live = true;
    call('/v1/weather/latest', { lat, lon })
      .then(({ status, body }) => {
        if (!live) return;
        setState(status === 200 ? { status: 'ok', o: body.observation } : { status: 'fail' });
      })
      .catch(() => live && setState({ status: 'fail' }));
    return () => {
      live = false;
    };
  }, [lat, lon]);

  const { status, o } = state;
  const specs =
    status === 'ok'
      ? [
          [fields.humidity, val(o.humidity_pct, 0), '%'],
          [fields.pressure, val(o.pressure_hpa), 'hPa'],
          [fields.wind, val(o.wind_ms), `m/s ${compass(o.wind_direction_deg)}`],
          [fields.rain, val(o.precipitation_1h_mm), 'mm'],
          [fields.sun, val(o.sun_1h_h), 'h'],
          [fields.snow, val(o.snow_cm, 0), 'cm'],
        ]
      : [];

  return (
    <div class="panel px-6 py-7 sm:px-8">
      <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
        <p className="label m-0">{s.live}</p>
        {status === 'loading' && <span className="spinner text-accent" aria-label={s.live} />}
        {status === 'ok' && (
          <p className="mono m-0 text-[0.76rem] text-muted">{jst(o.observed_at)} JST</p>
        )}
        {status === 'fail' && <p className="m-0 text-[0.82rem] text-warn">{t(lang).map.failed}</p>}
      </div>

      {status === 'ok' && (
        <>
          <p className="mono mt-4 mb-0 text-[clamp(2.6rem,7vw,4rem)] leading-none font-medium text-ink">
            {val(o.temp_c)}
            <span className="unit">°C</span>
          </p>
          <dl className="mt-8 grid grid-cols-2 gap-x-8 gap-y-6 sm:grid-cols-3">
            {specs.map(([label, value, unit]) => (
              <div key={label}>
                <dt className="label">{label}</dt>
                <dd
                  className={`mono mt-1 mb-0 text-[1.25rem] leading-none font-medium ${
                    value === '—' ? 'text-rule-2' : 'text-ink'
                  }`}
                >
                  {value}
                  {value !== '—' && <span className="unit">{unit}</span>}
                </dd>
              </div>
            ))}
          </dl>
          <p className="mt-7 mb-0 text-[0.78rem] text-muted">{s.liveNote}</p>
        </>
      )}
    </div>
  );
}
