package api

import (
	_ "embed"
	"net/http"
)

// The spec is embedded rather than read from disk so the container image stays
// a single binary and the docs cannot drift from the build that serves them.
//
//go:embed openapi.yaml
var openAPISpec []byte

// scalarVersion is pinned deliberately: @latest would let a CDN publish break
// the docs page of a build that has not changed.
const scalarVersion = "1.68.0"

// scalarSRI is the sha384 of the bundle that version resolves to. Pinning the
// version stops the CDN serving a *newer* script; the integrity hash stops it
// serving a *different* one. Recompute on any version bump:
//
//	curl -sL https://cdn.jsdelivr.net/npm/@scalar/api-reference@VERSION |
//	  openssl dgst -sha384 -binary | openssl base64 -A
const scalarSRI = "sha384-ayGz8N+NChlUEfR0zr5Zy3T6Q4lhcdiASJNoshS6+vxV56ZE300qfWNBjj9pqsLN"

// docsHTML loads Scalar from a CDN and points it at the spec below. The spec is
// referenced by relative URL so the page works on localhost and prod alike,
// with no server name baked in.
const docsHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>tenkinow</title>
</head>
<body>
<div id="app"></div>
<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@` + scalarVersion + `"
        integrity="` + scalarSRI + `" crossorigin="anonymous"></script>
<script>
  Scalar.createApiReference('#app', { url: '/v1/openapi.yaml' })
</script>
</body>
</html>
`

func (h *Handler) docs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Both docs responses are static per build, so a short TTL is enough to
	// absorb reloads while still picking up a deploy within minutes.
	w.Header().Set("Cache-Control", "public, max-age=300")

	if _, err := w.Write([]byte(docsHTML)); err != nil {
		h.log.Error("write docs", "error", err)
	}
}

func (h *Handler) openAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")

	if _, err := w.Write(openAPISpec); err != nil {
		h.log.Error("write openapi spec", "error", err)
	}
}
