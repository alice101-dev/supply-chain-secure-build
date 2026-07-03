package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestHandler() *Handler {
	return New(slog.New(slog.DiscardHandler), "v1.2.3", "abc1234")
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	rec := get(t, newTestHandler().Routes(), "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestReadyzFlipsDuringDrain(t *testing.T) {
	h := newTestHandler()
	routes := h.Routes()

	if rec := get(t, routes, "/readyz"); rec.Code != http.StatusOK {
		t.Fatalf("readyz before drain: got %d, want %d", rec.Code, http.StatusOK)
	}

	h.SetReady(false)
	if rec := get(t, routes, "/readyz"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz during drain: got %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestVersionReportsBuildIdentity(t *testing.T) {
	rec := get(t, newTestHandler().Routes(), "/version")
	if rec.Code != http.StatusOK {
		t.Fatalf("version: got %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("version: invalid JSON: %v", err)
	}
	if body["version"] != "v1.2.3" || body["commit"] != "abc1234" {
		t.Fatalf("version: got %v, want v1.2.3/abc1234", body)
	}
}

func TestUnknownPathIs404(t *testing.T) {
	rec := get(t, newTestHandler().Routes(), "/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown path: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}
