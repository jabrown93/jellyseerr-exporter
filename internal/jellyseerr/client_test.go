package jellyseerr

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestClientEndpoints(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/api/v1/auth/me":
			serveFixture(t, w, "auth-me.json")
		case "/api/v1/request":
			if got, want := r.URL.Query().Get("take"), "50"; got != want {
				t.Errorf("take = %q, want %q", got, want)
			}
			if got, want := r.URL.Query().Get("skip"), "50"; got != want {
				t.Errorf("skip = %q, want %q", got, want)
			}
			if got, want := r.URL.Query().Get("filter"), string(RequestFilterAll); got != want {
				t.Errorf("filter = %q, want %q", got, want)
			}
			if got, want := r.URL.Query().Get("sort"), string(RequestSortAdded); got != want {
				t.Errorf("sort = %q, want %q", got, want)
			}
			serveFixture(t, w, "requests-page-2.json")
		case "/api/v1/movie/603":
			if got, want := r.URL.Query().Get("language"), "en"; got != want {
				t.Errorf("language = %q, want %q", got, want)
			}
			serveFixture(t, w, "movie-603.json")
		case "/api/v1/tv/1396":
			serveFixture(t, w, "tv-1396.json")
		case "/api/v1/user":
			if got, want := r.URL.Query().Get("skip"), "0"; got != want {
				t.Errorf("skip = %q, want %q", got, want)
			}
			serveFixture(t, w, "users-page-1.json")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client, err := NewKeyAuth(srv.URL+"/", "en", "secret")
	if err != nil {
		t.Fatalf("NewKeyAuth: %v", err)
	}

	requests, page, err := client.GetRequests(1, 50, RequestFilterAll, RequestSortAdded)
	if err != nil {
		t.Fatalf("GetRequests: %v", err)
	}
	if page.Page != 2 || page.Pages != 2 || page.Results != 51 || len(requests) != 2 {
		t.Fatalf("GetRequests = %#v, page = %#v", requests, page)
	}
	if got, want := requests[0].Status.ToString(), "Approved"; got != want {
		t.Errorf("request status = %q, want %q", got, want)
	}
	if got, want := requests[0].Media.Status.ToString(), "Available"; got != want {
		t.Errorf("media status = %q, want %q", got, want)
	}
	if !requests[0].IsUHD {
		t.Error("movie request IsUHD = false, want true")
	}

	movie, err := requests[0].GetMovieDetails(client)
	if err != nil {
		t.Fatalf("GetMovieDetails: %v", err)
	}
	if got, want := movie.Genres[0].Name, "Action"; got != want {
		t.Errorf("movie genre = %q, want %q", got, want)
	}
	if got, want := movie.ProductionCompanies[0].Name, "Village Roadshow"; got != want {
		t.Errorf("movie company = %q, want %q", got, want)
	}
	if _, err := requests[0].GetTVDetails(client); err == nil {
		t.Error("GetTVDetails on movie request returned nil error")
	}

	tv, err := requests[1].GetTVDetails(client)
	if err != nil {
		t.Fatalf("GetTVDetails: %v", err)
	}
	if got, want := tv.Networks[0].Name, "HBO"; got != want {
		t.Errorf("TV network = %q, want %q", got, want)
	}
	if _, err := requests[1].GetMovieDetails(client); err == nil {
		t.Error("GetMovieDetails on TV request returned nil error")
	}

	users, userPage, err := client.GetAllUsers(50, 0)
	if err != nil {
		t.Fatalf("GetAllUsers: %v", err)
	}
	if userPage.Pages != 1 || len(users) != 1 || users[0].RequestCount != 3 {
		t.Errorf("GetAllUsers = %#v, page = %#v", users, userPage)
	}
}

func TestClientErrors(t *testing.T) {
	t.Run("bad API key", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()

		if _, err := NewKeyAuth(srv.URL, "en", "wrong"); err == nil {
			t.Error("NewKeyAuth with rejected API key returned nil error")
		}
	})

	t.Run("bad JSON", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/auth/me" {
				serveFixture(t, w, "auth-me.json")
				return
			}
			w.Write([]byte("not JSON"))
		}))
		defer srv.Close()

		client, err := NewKeyAuth(srv.URL, "en", "secret")
		if err != nil {
			t.Fatalf("NewKeyAuth: %v", err)
		}
		if _, _, err := client.GetRequests(0, 50, RequestFilterAll, RequestSortAdded); err == nil {
			t.Error("GetRequests with malformed JSON returned nil error")
		}
	})

	t.Run("API down", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL
		srv.Close()

		if _, err := NewKeyAuth(url, "en", "secret"); err == nil {
			t.Error("NewKeyAuth with unavailable API returned nil error")
		}
	})
}

func TestStatusStrings(t *testing.T) {
	for _, test := range []struct {
		name string
		got  string
		want string
	}{
		{"pending request", RequestStatusPending.ToString(), "Pending"},
		{"approved request", RequestStatusApproved.ToString(), "Approved"},
		{"declined request", RequestStatusDeclined.ToString(), "Declined"},
		{"available request", RequestStatusAvailable.ToString(), "Available"},
		{"unknown request", RequestStatus(99).ToString(), "Unknown"},
		{"pending media", MediaStatusPending.ToString(), "Pending"},
		{"processing media", MediaStatusProcessing.ToString(), "Processing"},
		{"partial media", MediaStatusPartial.ToString(), "Part-Available"},
		{"available media", MediaStatusAvailable.ToString(), "Available"},
		{"unknown media", MediaStatusUnknown.ToString(), "Unknown"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Errorf("ToString() = %q, want %q", test.got, test.want)
			}
		})
	}
}

func serveFixture(t *testing.T, w http.ResponseWriter, name string) {
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
