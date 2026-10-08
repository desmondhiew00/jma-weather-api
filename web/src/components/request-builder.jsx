import { useEffect, useRef, useState } from 'react';
import { API, call, locate } from '../lib/wx.js';
import { buildRequest, localValue } from '../lib/request.js';
import { t } from '../lib/i18n.js';

const ENDPOINTS = ['latest', 'history', 'at'];

const PRESETS = [
  { name: '東京 Tokyo', lat: '35.69', lon: '139.70' },
  { name: '大阪 Osaka', lat: '34.69', lon: '135.50' },
  { name: '札幌 Sapporo', lat: '43.06', lon: '141.35' },
  { name: '那覇 Naha', lat: '26.21', lon: '127.68' },
  { name: '富士山 Mt. Fuji', lat: '35.36', lon: '138.73' },
];

export default function RequestBuilder({ lang = 'en' }) {
  const s = t(lang).pg;
  const msg = t(lang).msg;

  const [endpoint, setEndpoint] = useState('latest');
  const [lat, setLat] = useState('35.69');
  const [lon, setLon] = useState('139.70');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [at, setAt] = useState('');

  const [state, setState] = useState('idle'); // idle | sending | done
  const [meta, setMeta] = useState({ tone: '', text: s.notSent });
  const [body, setBody] = useState('{\n  "station": { … },\n  "observation": { … }\n}');
  const [locating, setLocating] = useState(false);
  const [copied, setCopied] = useState(false);
  const copyTimer = useRef(null);

  // Default the time fields to a range that exists: the last six hours. Done on
  // mount rather than in the initial state, because the server prerender has no
  // business deciding what "now" is in the reader's timezone.
  useEffect(() => {
    const now = new Date();
    setTo(localValue(now));
    setAt(localValue(now));
    setFrom(localValue(new Date(now - 6 * 3600 * 1000)));
  }, []);

  useEffect(() => () => clearTimeout(copyTimer.current), []);

  useEffect(() => {
    const q = new URLSearchParams(location.search);
    if (q.has('lat')) setLat(q.get('lat'));
    if (q.has('lon')) setLon(q.get('lon'));
  }, []);

  const request = buildRequest({ endpoint, lat, lon, from, to, at }, API);

  async function onLocate() {
    setLocating(true);
    try {
      const here = await locate();
      setLat(String(here.lat));
      setLon(String(here.lon));
      setMeta({ tone: '', text: msg.located });
    } catch (err) {
      setMeta({ tone: '', text: err.message });
    } finally {
      setLocating(false);
    }
  }

  async function onSubmit(e) {
    e.preventDefault();
    setState('sending');
    try {
      const res = await call(request.path, request.params);
      setMeta({ tone: res.status === 200 ? 'ok' : 'bad', code: res.status, text: `${res.ms} ms` });
      setBody(JSON.stringify(res.body, null, 2));
    } catch {
      setMeta({ tone: 'bad', code: 'network error', text: msg.networkError });
    } finally {
      setState('done');
    }
  }

  async function onCopy() {
    await navigator.clipboard.writeText(`curl -s '${request.url}' | jq`);
    setCopied(true);
    copyTimer.current = setTimeout(() => setCopied(false), 1500);
  }

  const sending = state === 'sending';
  const sent = Boolean(meta.code);

  return (
    /* Split Studio: the controls on one side, what they produce on the other.
       Below 64rem the pair stacks, form first, because you cannot read a
       response you have not asked for yet. */
    <div className="grid items-start gap-x-14 gap-y-10 lg:grid-cols-[minmax(19rem,24rem)_minmax(0,1fr)]">
      <form onSubmit={onSubmit}>
        <fieldset className="m-0 mb-7 border-0 p-0">
          <legend className="p-0">{s.endpoint}</legend>
          <div className="flex flex-wrap gap-2">
            {ENDPOINTS.map((name) => (
              <button
                key={name}
                className="chip"
                type="button"
                aria-pressed={endpoint === name}
                onClick={() => setEndpoint(name)}
              >
                {name}
              </button>
            ))}
          </div>
        </fieldset>

        <fieldset className="m-0 mb-7 border-0 border-t border-t-rule p-0 pt-6">
          <legend className="p-0">{s.coordinate}</legend>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="lat">lat</label>
              <input
                id="lat"
                className="field min-w-0"
                value={lat}
                onChange={(e) => setLat(e.target.value)}
                inputMode="decimal"
              />
            </div>
            <div>
              <label htmlFor="lon">lon</label>
              <input
                id="lon"
                className="field min-w-0"
                value={lon}
                onChange={(e) => setLon(e.target.value)}
                inputMode="decimal"
              />
            </div>
          </div>
          <p className="mt-3 mb-0 text-[0.78rem] text-muted">{s.coordHelp}</p>
          <div className="mt-4 flex flex-wrap gap-2">
            <button
              className="chip gap-2"
              type="button"
              onClick={onLocate}
              disabled={locating}
              data-state={locating ? 'loading' : undefined}
            >
              {locating && <span className="spinner" aria-hidden="true" />}
              {s.here}
            </button>
            {PRESETS.map((p) => (
              <button
                key={p.name}
                className="chip"
                type="button"
                onClick={() => {
                  setLat(p.lat);
                  setLon(p.lon);
                }}
              >
                {p.name}
              </button>
            ))}
          </div>
        </fieldset>

        {endpoint === 'history' && (
          <fieldset className="m-0 mb-7 border-0 border-t border-t-rule p-0 pt-6">
            <legend className="p-0">{s.range}</legend>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label htmlFor="from">from</label>
                <input
                  id="from"
                  type="datetime-local"
                  className="field min-w-0"
                  value={from}
                  onChange={(e) => setFrom(e.target.value)}
                />
              </div>
              <div>
                <label htmlFor="to">to</label>
                <input
                  id="to"
                  type="datetime-local"
                  className="field min-w-0"
                  value={to}
                  onChange={(e) => setTo(e.target.value)}
                />
              </div>
            </div>
            <p className="mt-3 mb-0 text-[0.78rem] text-muted">{s.rangeHelp}</p>
          </fieldset>
        )}

        {endpoint === 'at' && (
          <fieldset className="m-0 mb-7 border-0 border-t border-t-rule p-0 pt-6">
            <legend className="p-0">{s.moment}</legend>
            <label htmlFor="at">at</label>
            <input
              id="at"
              type="datetime-local"
              className="field"
              value={at}
              onChange={(e) => setAt(e.target.value)}
            />
            <p className="mt-3 mb-0 text-[0.78rem] text-muted">{s.momentHelp}</p>
          </fieldset>
        )}

        <div className="flex flex-wrap items-center gap-3 border-t border-rule pt-6">
          <button
            className="btn"
            type="submit"
            disabled={sending}
            data-state={sending ? 'loading' : undefined}
          >
            {sending && <span className="spinner" aria-hidden="true" />}
            {s.send}
          </button>
          <button
            className="chip px-4"
            type="button"
            onClick={onCopy}
            data-state={copied ? 'success' : undefined}
          >
            {copied ? s.copied : s.copy}
          </button>
          {/* Announced, not shown: the button's own label already says it visually. */}
          <span className="absolute h-px w-px overflow-hidden [clip-path:inset(50%)]" role="status">
            {copied ? s.copiedSaid : ''}
          </span>
        </div>
      </form>

      <div
        className={`panel overflow-hidden transition-opacity duration-200 ease-out lg:sticky lg:top-24 ${
          sending ? 'opacity-60' : 'opacity-100'
        }`}
        aria-busy={sending || undefined}
      >
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1 border-b border-rule px-4 py-3">
          <span className="mono rounded-full bg-accent-wash px-2 py-0.5 text-[0.62rem] tracking-[0.1em] text-accent uppercase">
            GET
          </span>
          <code className="min-w-0 break-all text-[0.82rem] text-ink">{request.url}</code>
        </div>

        <div
          className="mono flex items-baseline gap-2 border-b border-rule px-4 py-2 text-[0.76rem] text-muted"
          role="status"
        >
          {sent ? (
            <>
              <b className={meta.tone === 'ok' ? 'text-good' : 'text-warn'}>{meta.code}</b>
              <span aria-hidden="true">·</span>
              <span>{meta.text}</span>
            </>
          ) : (
            <span>{meta.text}</span>
          )}
        </div>

        <pre
          key={body}
          className={`mono m-0 max-h-[30rem] overflow-auto p-5 text-[0.8rem] leading-[1.7] motion-safe:animate-rise ${
            sent ? 'text-ink-2' : 'text-muted'
          }`}
          tabIndex={0}
        >
          {body}
        </pre>
      </div>
    </div>
  );
}
