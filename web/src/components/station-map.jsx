import { useEffect, useRef, useState } from 'react';
import { call, val, compass, jst } from '../lib/wx.js';
import { t } from '../lib/i18n.js';
import { MODES, colour, wash, legend, reading } from '../lib/map-scales.js';
import { BASEMAP_STYLES, CHIP_ZOOM, QUIET_STYLES, createDotsOverlay } from '../lib/dots-overlay.js';

/* Where a phone opens. The whole network framed into a 390px-wide band leaves
   the archipelago the size of a thumbnail with the Philippines in shot: Honshu
   is where most of the stations and most of the readers are, and Hokkaido and
   Kyushu are one pinch away. A wide screen has the room to open on the network
   itself, so it still does. */
const HONSHU = { south: 33.8, west: 130.6, north: 41.6, east: 142.2 };

/* A mark per mode, for the phone, where three words in a row is most of a
   panel. Drawn on the same 15-unit grid as the search magnifier and in
   currentColor, so a pressed chip tints its icon with its text. */
const MODE_ICON = {
  temp: (
    <>
      <path d="M6 8.8V3a1.5 1.5 0 0 1 3 0v5.8" stroke="currentColor" strokeWidth="1.4" />
      <circle cx="7.5" cy="10.6" r="2.6" stroke="currentColor" strokeWidth="1.4" />
    </>
  ),
  rain: (
    <path
      d="M7.5 1.8c1.8 2.1 3.4 4.2 3.4 5.9a3.4 3.4 0 0 1-6.8 0c0-1.7 1.6-3.8 3.4-5.9z"
      stroke="currentColor"
      strokeWidth="1.4"
    />
  ),
  wind: (
    <>
      <path d="M1.5 5.5h6.2a1.9 1.9 0 1 0-1.9-1.9" stroke="currentColor" strokeWidth="1.4" />
      <path d="M1.5 9.5h8a1.9 1.9 0 1 1-1.9 1.9" stroke="currentColor" strokeWidth="1.4" />
    </>
  ),
};

/** The three readings the card shows beneath the headline value. */
function cardRows(mode, o, M) {
  if (mode === 'wind') {
    // No station picked at all reads as a dash; a picked station that reports no
    // direction is calm. An empty card must not claim the weather is calm.
    const dir = o.wind_direction_deg;
    return [
      [M.modes.temp, val(o.temp_c)],
      ['dir', dir === undefined ? '—' : dir === null ? M.calm : compass(dir)],
      [M.modes.rain, val(o.precipitation_1h_mm)],
    ];
  }
  return [
    [M.modes.temp, val(o.temp_c)],
    [M.modes.rain, val(o.precipitation_1h_mm)],
    [M.modes.wind, val(o.wind_ms)],
  ];
}

export default function StationMap({ lang = 'en', mapsKey = '' }) {
  const S = t(lang);
  const M = S.map;

  const [stations, setStations] = useState([]);
  const [mode, setMode] = useState('temp');
  const [meta, setMeta] = useState(M.loading);
  const [card, setCard] = useState(null); // the station the readout is showing
  // Whether the search field is open. It only ever closes on a phone, where an
  // always-open field, its label and its help line cost a third of the map for
  // a thing most readers never type in: they tap a dot.
  const [finding, setFinding] = useState(false);

  const canvasEl = useRef(null);
  const findEl = useRef(null);
  const mapRef = useRef(null);
  const overlayRef = useRef(null);

  // What the canvas overlay reads when it redraws. A ref because the overlay is
  // a Maps class living outside React's render cycle.
  const live = useRef({ stations: [], mode });
  live.current = { stations, mode, active: card?.id };

  useEffect(() => {
    if (!mapsKey) {
      setMeta('GOOGLE_MAP_API is not set; the map cannot load.');
      return;
    }

    let cancelled = false;

    (async () => {
      const snapshot = await call('/v1/weather/now', {}).catch(() => null);
      if (cancelled) return;
      if (!snapshot || snapshot.status !== 200) {
        setMeta(M.failed);
        return;
      }

      const loaded = snapshot.body.stations;
      setStations(loaded);
      setMeta(M.counted(snapshot.body.count, jst(snapshot.body.observed_at)));

      const maps = await google.maps.importLibrary('maps');
      if (cancelled) return;

      const map = new maps.Map(canvasEl.current, {
        center: { lat: 37.5, lng: 137.5 },
        zoom: 5,
        minZoom: 4,
        disableDefaultUI: true,
        zoomControl: true,
        // On a phone the reading sheet is pinned across the bottom, which is
        // where Maps puts its zoom control by default: move it up the right
        // edge so the sheet does not sit on top of it.
        zoomControlOptions: {
          position: window.matchMedia('(pointer: coarse)').matches
            ? google.maps.ControlPosition.RIGHT_CENTER
            : google.maps.ControlPosition.RIGHT_BOTTOM,
        },
        // Greedy everywhere: this page is one fixed screen with nothing under
        // it to scroll to, so there is no page scroll for the map to steal and
        // no reason to make a reader pan with two fingers.
        gestureHandling: 'greedy',
        // The API rests on an openhand cursor; google.com/maps uses an arrow.
        // Match the arrow. draggingCursor is left unset, so panning still shows
        // the closed hand and a drag still reads as a drag.
        draggableCursor: 'default',
        // Maps JS takes a parsed colour, not a custom property, so the night
        // water is repeated here by hand. Keep it in step with dots-overlay.js.
        backgroundColor: '#121724',
        styles: BASEMAP_STYLES,
      });
      mapRef.current = map;

      // Close in, the dots turn into chips carrying their reading, and the
      // basemap's own labels step back so the numbers read first.
      let quiet = false;
      map.addListener('zoom_changed', () => {
        const near = map.getZoom() >= CHIP_ZOOM;
        if (near === quiet) return;
        quiet = near;
        map.setOptions({ styles: near ? QUIET_STYLES : BASEMAP_STYLES });
      });

      // google.maps, not the imported handle: the overlay also needs LatLng,
      // which lives in core rather than the maps library.
      const overlay = createDotsOverlay(google.maps, () => live.current);
      overlay.setMap(map);
      overlayRef.current = overlay;

      // Frame the network itself rather than a hardcoded view: if JMA adds or
      // retires stations at the edges, the opening view follows. An empty
      // snapshot leaves the box empty, and fitBounds on an empty box puts the
      // map at an unrenderable view, so keep the opening centre instead.
      const wide = window.matchMedia('(min-width: 64rem)').matches;

      if (!wide) {
        // The head is one chip row at the top of the phone view and the reading
        // sheet sits at the bottom, so Honshu is framed into the band left
        // between them.
        map.fitBounds(HONSHU, { top: 76, right: 16, bottom: 148, left: 16 });
      } else if (loaded.length) {
        const box = new google.maps.LatLngBounds();
        for (const st of loaded) box.extend({ lat: st.lat, lng: st.lon });

        // Fit the network to the part of the map nothing is covering: the
        // controls sit over the left of a full-bleed canvas, so fitting to the
        // whole canvas would frame the network behind them and leave the open
        // water on the right unused.
        map.fitBounds(box, { top: 64, right: 56, bottom: 104, left: 456 });
      }
    })();

    return () => {
      cancelled = true;
      overlayRef.current?.setMap(null);
    };
  }, [mapsKey]);

  // Opening the search puts the caret in it: the button was the reader asking
  // to type. React's autoFocus does nothing here because the input is mounted
  // all along, only hidden.
  useEffect(() => {
    if (finding) findEl.current?.focus();
  }, [finding]);

  // Switching mode recolours the dots already on screen; picking a station moves
  // the ring onto it.
  useEffect(() => {
    overlayRef.current?.draw();
  }, [mode, stations, card]);

  // Native listeners rather than React's: Maps JS stops pointer events inside
  // the map, so a delegated handler at the React root would never see them.
  // Selection is a click, not a hover: a pointerdown that travels more than a
  // few pixels is a pan of the map, not a choice of station. The cursor is left
  // to Maps.
  useEffect(() => {
    const node = canvasEl.current;
    if (!node) return;

    let from = null;

    const onDown = (e) => {
      from = { x: e.clientX, y: e.clientY };
    };
    const onUp = (e) => {
      if (!from) return;
      const moved = Math.hypot(e.clientX - from.x, e.clientY - from.y);
      from = null;
      if (moved > 6) return; // a pan, not a click

      const st = overlayRef.current?.pick(e);
      if (st) setCard(st);
    };
    const onCancel = () => {
      from = null;
    };

    node.addEventListener('pointerdown', onDown);
    node.addEventListener('pointerup', onUp);
    node.addEventListener('pointercancel', onCancel);
    return () => {
      node.removeEventListener('pointerdown', onDown);
      node.removeEventListener('pointerup', onUp);
      node.removeEventListener('pointercancel', onCancel);
    };
  }, []);

  function onFind(e) {
    const q = e.target.value.trim().toLowerCase();
    const hit = stations.find((s) => `${s.name} ${s.name_en}`.toLowerCase() === q);
    if (!hit || !mapRef.current) return;
    setCard(hit);
    // The field has done its job, and on a phone it is sitting over the map it
    // just pointed at.
    setFinding(false);
    mapRef.current.panTo({ lat: hit.lat, lng: hit.lon });
    mapRef.current.setZoom(Math.max(mapRef.current.getZoom(), 8));
  }

  const bar = legend(mode);
  const o = card?.observation ?? {};

  return (
    /* The map is the page, on every screen: a fixed viewport-high layer with
       the canvas filling it and everything else floating over it in one
       column: the controls resting at the top, the reading at the bottom.
       Nothing sits below it, so the page does not scroll and a single finger
       is free to pan the map.
       The controls come first and never move. The card is conditional, and a
       card placed above the field would shove the field down on every first
       search. */
    <section className="fixed inset-0 grid grid-rows-[auto_1fr_auto] bg-deep-2 px-[var(--page-gutter)] pt-[var(--nav-space)] pb-[max(0.75rem,env(safe-area-inset-bottom))] lg:static lg:mt-[calc(var(--nav-space)*-1)] lg:h-[100dvh] lg:grid-cols-[25rem_1fr] lg:grid-rows-[auto_auto] lg:content-end lg:items-start lg:gap-y-3 lg:px-6 lg:pt-[calc(var(--nav-space)+0.75rem)] lg:pb-6">
      {/* On a phone the head hangs off the right edge, the side the hand
          holding the phone is on, and the side the reading sheet below it does
          not start from. On a wide screen it is the top of the left column. */}
      <div className="pointer-events-none relative z-20 row-start-1 flex justify-end lg:col-start-1 lg:block">
        {/* Shut, the head is four marks wide and no wider: a panel spanning the
            screen to hold four icons is just a lid on the map. Opening it takes
            the full width, because a search field needs it. */}
        <div
          className={`instrument pointer-events-auto p-4 sm:p-5 lg:w-[25rem] lg:p-5 ${
            finding ? '' : 'max-lg:w-fit max-lg:p-1.5'
          }`}
        >
          <h1 className="sr-only">{M.h2}</h1>
          <p className="sr-only">{M.lede}</p>

          {/* On a phone the field lives behind the search button below; from
              lg up there is room to show it. */}
          <div
            className={
              finding
                ? 'max-lg:mb-3.5 max-lg:border-b max-lg:border-white/10 max-lg:pb-3.5'
                : 'max-lg:hidden'
            }
          >
            <label htmlFor="find" className="max-lg:sr-only">
              {M.find}
            </label>
            <input
              id="find"
              className="field"
              list="stations"
              autoComplete="off"
              ref={findEl}
              aria-describedby="find-help"
              onChange={onFind}
            />
            <datalist id="stations">
              {stations.map((s) => (
                <option key={s.id} value={`${s.name} ${s.name_en}`} />
              ))}
            </datalist>
            <p id="find-help" className="mt-1.5 mb-0 text-[0.72rem] text-rule-2 max-sm:hidden">
              {M.findHelp}
            </p>
          </div>

          {/* What the dots are painting, and (on a phone, at the head of the
              same row) the way into the search field. The row is the whole
              panel when the field is shut, which is how the map gets the top of
              the screen back. */}
          <div className="no-bar flex items-center gap-2 overflow-x-auto lg:mt-4 lg:flex-wrap lg:overflow-visible lg:border-t lg:border-white/10 lg:pt-4">
            <button
              className="chip size-8 min-h-8 justify-center p-0 lg:hidden"
              type="button"
              aria-expanded={finding}
              aria-controls="find"
              aria-label={M.find}
              onClick={() => setFinding((v) => !v)}
            >
              {/* A magnifier, not the words: "Find a station" beside three mode
                  chips wraps to a second row on a 360px screen. */}
              <svg
                width="15"
                height="15"
                viewBox="0 0 15 15"
                fill="none"
                strokeLinecap="round"
                aria-hidden="true"
              >
                <circle cx="6.5" cy="6.5" r="4.75" stroke="currentColor" strokeWidth="1.4" />
                <path d="M10.2 10.2 13.5 13.5" stroke="currentColor" strokeWidth="1.4" />
              </svg>
            </button>
            {MODES.map((name) => (
              <button
                key={name}
                className="chip max-sm:size-8 max-sm:min-h-8 max-sm:justify-center max-sm:p-0"
                type="button"
                aria-pressed={mode === name}
                onClick={() => setMode(name)}
              >
                <svg
                  className="sm:hidden"
                  width="15"
                  height="15"
                  viewBox="0 0 15 15"
                  fill="none"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                >
                  {MODE_ICON[name]}
                </svg>
                <span className="max-sm:sr-only">{M.modes[name]}</span>
              </button>
            ))}
          </div>
          <div
            className={`mono mt-2.5 text-[0.72rem] text-rule-2 [@media(height<44rem)]:hidden ${
              finding ? '' : 'max-lg:hidden'
            }`}
            role="status"
          >
            {meta}
          </div>
        </div>
      </div>

      {/* The map and its legend. On a wide screen this is the layer the rest of
          the page sits on, so it is taken out of the grid and pinned to the
          whole section, ignoring the padding the column is inset by. The legend
          is the one thing that stays on the right, clear of that column. */}
      <div className="absolute inset-0 z-0">
        <div
          className="absolute inset-0 bg-deep-2"
          ref={canvasEl}
          role="application"
          aria-label={M.h2}
        />

        <div
          /* Maps puts its zoom control in the bottom-right corner, so the
             legend stops short of it rather than sitting on top of it. */
          className="instrument mono absolute right-[var(--page-gutter)] bottom-[max(0.75rem,env(safe-area-inset-bottom))] z-10 flex items-center gap-3 px-3.5 py-2 text-[0.68rem] text-rule-2 max-lg:left-[var(--page-gutter)] max-lg:justify-between lg:right-[4.75rem] lg:bottom-6"
          aria-hidden="true"
        >
          <span>{mode === 'rain' ? M.noRain : bar.lo}</span>
          <span className="flex overflow-hidden rounded-full">
            {bar.swatches.map((c, i) => (
              <i key={i} className="block h-[9px] w-[13px]" style={{ background: c }} />
            ))}
          </span>
          <span>{`${bar.hi} ${M.units[mode]}`}</span>
        </div>
      </div>

      {/* The readout follows the pointer's nearest station; it is also what the
          station search writes into, so the map is never the only way in. It is
          always on screen, one line per field: an empty card holding its place
          beats a card that appears and shoves the field it was filled from.
          It wears the same pale surface as the controls above it, so the two
          read as one column rather than as a panel and a foreign slab; the only
          colour in it is the reading's own place on the mode's scale.
          Keyed on the station so each pick replays the entrance. */}
      <aside
        key={card?.id ?? 'empty'}
        className="instrument pointer-events-none relative z-20 row-start-3 overflow-hidden motion-safe:animate-rise max-lg:mb-11 lg:col-start-1 lg:row-start-2 lg:mb-0 lg:w-full"
      >
        {card ? (
          <>
            <div className="flex items-baseline justify-between gap-3 px-4 pt-2.5 sm:px-5 sm:pt-4">
              <div className="font-display min-w-0 text-[1.3rem] leading-tight font-bold text-paper">
                {card.name}
                <span className="mono ml-2 text-[0.75rem] font-normal text-rule-2">
                  {card.name_en}
                </span>
              </div>
              <span className="mono flex-none text-[0.7rem] text-rule-2">{card.id}</span>
            </div>

            {/* The value, sat on the wash of its own place in the scale, with
                the station's dot beside it in the same colour, at the same
                reading, as the mark the reader just clicked on the map. The
                number itself stays near-white: the middle of every ramp is
                deliberately recessive, and a headline cannot be. */}
            <div
              className="mt-2 flex items-baseline gap-2.5 px-4 py-1.5 sm:mt-3.5 sm:px-5 sm:py-3"
              style={{ background: wash(mode, reading(mode, o)) }}
            >
              <i
                className="mb-1.5 size-2.5 flex-none self-end rounded-full"
                style={{ background: colour(mode, reading(mode, o)) }}
                aria-hidden="true"
              />
              <span className="mono text-[1.75rem] leading-none font-medium tracking-[-0.04em] text-paper sm:text-[2.6rem] [@media(height<44rem)]:text-[1.6rem]">
                {val(reading(mode, o))}
              </span>
              <span className="mono text-[0.8rem] text-rule-2">{M.units[mode]}</span>
              <span className="mono ml-auto self-end text-[0.7rem] text-rule-2">
                {`${jst(o.observed_at)} JST`}
              </span>
            </div>

            <dl className="m-0 grid grid-cols-3 gap-3 px-4 py-2 sm:px-5 sm:py-3.5 [@media(height<44rem)]:py-1.5">
              {cardRows(mode, o, M).map(([label, value]) => (
                <div key={label}>
                  <dt className="label text-[0.58rem] leading-tight">{label}</dt>
                  <dd
                    className={`mono mt-0.5 mb-0 text-[0.9rem] sm:mt-1 sm:text-[0.95rem] ${
                      value === '—' ? 'text-muted' : 'text-paper'
                    }`}
                  >
                    {value}
                  </dd>
                </div>
              ))}
            </dl>
          </>
        ) : (
          /* An empty card shows a hint instead of a row of dashes. It keeps the card's
             height so the first pick does not shift the column. */
          <p className="m-0 flex items-center px-4 py-3.5 text-[0.9rem] text-rule-2 sm:px-5 sm:py-4 lg:min-h-[9.5rem] lg:py-0">
            {M.hint}
          </p>
        )}
      </aside>
    </section>
  );
}
