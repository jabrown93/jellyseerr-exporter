package collector

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/opspotes/jellyseerr-exporter/internal/jellyseerr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRequestCollectorFullDataPaginatesAndLabelsMetrics(t *testing.T) {
	var mu sync.Mutex
	var skips []string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/request":
			mu.Lock()
			skips = append(skips, r.URL.Query().Get("skip"))
			mu.Unlock()
			switch r.URL.Query().Get("skip") {
			case "0":
				serveCollectorFixture(t, w, "requests-page-0.json")
			case "50":
				serveCollectorFixture(t, w, "requests-page-1.json")
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
		case "/api/v1/movie/603":
			serveCollectorFixture(t, w, "movie-603.json")
		case "/api/v1/tv/1396":
			serveCollectorFixture(t, w, "tv-1396.json")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	collector := NewRequestCollector(client, true)
	expected := `
# HELP jellyseerr_requests_count Number of requests on Jellyseerr
# TYPE jellyseerr_requests_count gauge
jellyseerr_requests_count{company="HBO",genre="Drama",is_4k="false",media_status="Processing",media_type="tv",request_status="Pending"} 1
jellyseerr_requests_count{company="Village Roadshow",genre="Action",is_4k="true",media_status="Available",media_type="movie",request_status="Approved"} 1
# HELP jellyseerr_up Whether the last Jellyseerr API query for this collector succeeded (1) or failed (0).
# TYPE jellyseerr_up gauge
jellyseerr_up{collector="requests"} 1
`
	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected)); err != nil {
		t.Fatalf("CollectAndCompare: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if want := []string{"0", "50"}; !reflect.DeepEqual(skips, want) {
		t.Errorf("request page skips = %v, want %v", skips, want)
	}
}

func TestRequestCollectorWithoutDetailsUsesDefaultLabels(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/request" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		serveCollectorFixture(t, w, "requests-single-page.json")
	})

	collector := NewRequestCollector(client, false)
	expected := `
# HELP jellyseerr_requests_count Number of requests on Jellyseerr
# TYPE jellyseerr_requests_count gauge
jellyseerr_requests_count{company="not_collected",genre="not_collected",is_4k="true",media_status="Available",media_type="movie",request_status="Approved"} 1
# HELP jellyseerr_up Whether the last Jellyseerr API query for this collector succeeded (1) or failed (0).
# TYPE jellyseerr_up gauge
jellyseerr_up{collector="requests"} 1
`
	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected)); err != nil {
		t.Fatalf("CollectAndCompare: %v", err)
	}
}

func TestUserCollectorPaginatesAndLabelsMetrics(t *testing.T) {
	var mu sync.Mutex
	var skips []string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		mu.Lock()
		skips = append(skips, r.URL.Query().Get("skip"))
		mu.Unlock()
		switch r.URL.Query().Get("skip") {
		case "0":
			serveCollectorFixture(t, w, "users-page-0.json")
		case "50":
			serveCollectorFixture(t, w, "users-page-1.json")
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	collector := NewUserCollector(client)
	expected := `
# HELP jellyseerr_up Whether the last Jellyseerr API query for this collector succeeded (1) or failed (0).
# TYPE jellyseerr_up gauge
jellyseerr_up{collector="users"} 1
# HELP jellyseerr_user_requests Number of requests made by a user
# TYPE jellyseerr_user_requests gauge
jellyseerr_user_requests{email="alice@example.com"} 3
jellyseerr_user_requests{email="bob@example.com"} 7
`
	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected)); err != nil {
		t.Fatalf("CollectAndCompare: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if want := []string{"0", "50"}; !reflect.DeepEqual(skips, want) {
		t.Errorf("user page skips = %v, want %v", skips, want)
	}
}

func TestCollectorsReportListAPIFailures(t *testing.T) {
	for _, failure := range []struct {
		name  string
		serve func(http.ResponseWriter)
	}{
		{"server error", func(w http.ResponseWriter) { w.WriteHeader(http.StatusInternalServerError) }},
		{"malformed JSON", func(w http.ResponseWriter) { _, _ = w.Write([]byte("{")) }},
		{"unauthorized", func(w http.ResponseWriter) { w.WriteHeader(http.StatusUnauthorized) }},
	} {
		for _, collector := range collectorFactories {
			t.Run(failure.name+"/"+collector.name, func(t *testing.T) {
				client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
					failure.serve(w)
				})
				assertCollectorDown(t, collector.new(client), collector.name)
			})
		}
	}
}

func TestCollectorsReportLaterPageFailures(t *testing.T) {
	for _, collector := range collectorFactories {
		t.Run(collector.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("skip") == "0" {
					serveCollectorFixture(t, w, collector.firstPage)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
			})
			assertCollectorDown(t, collector.new(client), collector.name)
		})
	}
}

func TestCollectorsRegisterUpMetric(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pageInfo":{"pages":1},"results":[]}`))
	})
	registry := prometheus.NewPedanticRegistry()
	if err := registry.Register(NewRequestCollector(client, false)); err != nil {
		t.Fatalf("register request collector: %v", err)
	}
	if err := registry.Register(NewUserCollector(client)); err != nil {
		t.Fatalf("register user collector: %v", err)
	}
	if _, err := registry.Gather(); err != nil {
		t.Fatalf("gather collectors: %v", err)
	}
}

type collectorFactory struct {
	name      string
	firstPage string
	new       func(*jellyseerr.Client) prometheus.Collector
}

var collectorFactories = []collectorFactory{
	{"requests", "requests-page-0.json", func(client *jellyseerr.Client) prometheus.Collector { return NewRequestCollector(client, false) }},
	{"users", "users-page-0.json", func(client *jellyseerr.Client) prometheus.Collector { return NewUserCollector(client) }},
}

func assertCollectorDown(t *testing.T, collector prometheus.Collector, name string) {
	t.Helper()
	expected := `
# HELP jellyseerr_up Whether the last Jellyseerr API query for this collector succeeded (1) or failed (0).
# TYPE jellyseerr_up gauge
jellyseerr_up{collector="` + name + `"} 0
`
	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected)); err != nil {
		t.Fatalf("CollectAndCompare: %v", err)
	}
}

func newTestClient(t *testing.T, api http.HandlerFunc) *jellyseerr.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/api/v1/auth/me" {
			serveCollectorFixture(t, w, "auth-me.json")
			return
		}
		api(w, r)
	}))
	t.Cleanup(srv.Close)

	client, err := jellyseerr.NewKeyAuth(srv.URL, "en", "secret")
	if err != nil {
		t.Fatalf("NewKeyAuth: %v", err)
	}
	return client
}

func serveCollectorFixture(t *testing.T, w http.ResponseWriter, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Errorf("read fixture %q: %v", name, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(data); err != nil {
		t.Errorf("write fixture %q: %v", name, err)
	}
}
