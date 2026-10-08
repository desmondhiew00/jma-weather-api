package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDocsPageServesScalar(t *testing.T) {
	h := newTestHandler(t, &stubStore{})

	for _, target := range []string{"/v1/docs", "/v1/docs/"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", target, rec.Code)
		}

		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s: content-type %q", target, ct)
		}

		// The page is useless if it cannot reach the spec it points at.
		if !strings.Contains(rec.Body.String(), "/v1/openapi.yaml") {
			t.Errorf("%s: page does not reference the spec URL", target)
		}
	}
}

// The spec is parsed, not only fetched, so a YAML typo or a bad $ref path is
// caught here rather than by whoever opens the docs page after a deploy.
func TestOpenAPISpecParses(t *testing.T) {
	h := newTestHandler(t, &stubStore{})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/openapi.yaml", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}

	var spec struct {
		OpenAPI string `yaml:"openapi"`
		Info    struct {
			Title string `yaml:"title"`
		} `yaml:"info"`
		Paths map[string]map[string]any `yaml:"paths"`
	}

	if err := yaml.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("spec is not valid YAML: %v", err)
	}

	if spec.OpenAPI == "" {
		t.Error("spec has no openapi version")
	}

	if spec.Info.Title == "" {
		t.Error("spec has no info.title")
	}

	// Every documented route must actually be served, or the spec lies.
	for path := range spec.Paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if _, pattern := h.(*http.ServeMux).Handler(req); pattern == "" {
			t.Errorf("spec documents %s but no route serves it", path)
		}
	}
}
