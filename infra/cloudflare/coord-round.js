// Rounds lat/lon in the query string before the edge looks up the cache.
//
// The cache key is the full URL, so an unrounded coordinate gives every caller
// their own cache entry and nothing is ever a HIT. That made a fresh
// request cost ~480 ms from Japan against ~150 ms for a cached one. Clients
// send GPS precision; the handler only uses the coordinate to pick the nearest
// station, so the extra digits buy nothing and cost every request.
//
// Two decimal places is ~1.1 km, well inside the spacing of the JMA station
// network, so the station this resolves to is unchanged. If stations ever get
// dense enough for that to matter, raise it to 3 (~110 m) rather than dropping
// the rounding; the hit rate scales with how coarse this is.
//
// Cloudflare does not re-invoke a Worker for its own subrequest, so rewriting
// the URL and re-fetching cannot loop.
addEventListener("fetch", (event) => {
  const url = new URL(event.request.url)
  let rewritten = false

  for (const key of ["lat", "lon"]) {
    const raw = url.searchParams.get(key)
    if (raw === null) continue

    const value = Number(raw)
    // Leave anything unparseable alone: the handler returns a better error for
    // it than a rewrite would, and rounding NaN would turn a 400 into a 200.
    if (!Number.isFinite(value)) continue

    url.searchParams.set(key, value.toFixed(2))
    rewritten = true
  }

  event.respondWith(rewritten ? fetch(new Request(url, event.request)) : fetch(event.request))
})
