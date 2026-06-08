package paneldns

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestProvider(t *testing.T, mux *http.ServeMux) *DNSProvider {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfg := NewDefaultConfig()
	cfg.APIURL = srv.URL
	cfg.APIToken = "test-token"
	cfg.HTTPClient = srv.Client()

	p, err := NewDNSProviderConfig(cfg)
	if err != nil {
		t.Fatalf("NewDNSProviderConfig: %v", err)
	}
	return p
}

func TestPresent(t *testing.T) {
	mux := http.NewServeMux()

	// Zone lookup
	mux.HandleFunc("/api/v1/zones", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("name") != "example.com" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []any{}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"data": []any{map[string]any{"id": 42, "name": "example.com"}},
		})
	})

	// Record create
	mux.HandleFunc("/api/v1/zones/42/records", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": map[string]any{"id": 99}})
	})

	p := newTestProvider(t, mux)
	if err := p.Present("example.com", "token", "keyAuth"); err != nil {
		t.Fatalf("Present: %v", err)
	}
}

func TestCleanUp(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/zones", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"data": []any{map[string]any{"id": 42, "name": "example.com"}},
		})
	})

	deleted := false
	mux.HandleFunc("/api/v1/zones/42/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": []any{
				map[string]any{"id": 7, "type": "TXT", "content": "some-other-value"},
			},
		})
	})

	mux.HandleFunc("/api/v1/zones/42/records/", func(w http.ResponseWriter, r *http.Request) {
		deleted = true
		w.WriteHeader(http.StatusNoContent)
	})

	p := newTestProvider(t, mux)
	// CleanUp when record not found should succeed silently
	if err := p.CleanUp("example.com", "token", "keyAuth"); err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
	if deleted {
		t.Error("expected no delete call when record not found")
	}
}

func TestNewDNSProvider_MissingToken(t *testing.T) {
	t.Setenv("PANELDNS_TOKEN", "")
	_, err := NewDNSProvider()
	if err == nil {
		t.Fatal("expected error when token is missing")
	}
}
