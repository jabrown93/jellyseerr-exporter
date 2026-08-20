package jellyseerr

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/auth/me":
			w.Write([]byte(`{"id":1,"email":"admin@example.com"}`))
		case "/api/v1/request":
			if got := r.URL.Query().Get("skip"); got != "50" {
				t.Errorf("skip = %q, want 50", got)
			}
			w.Write([]byte(`{
				"pageInfo": {"page": 2, "pages": 2, "results": 51},
				"results": [{"id": 7, "status": 2, "is4k": true,
					"media": {"tmdbId": 603, "mediaType": "movie", "status": 5}}]
			}`))
		case "/api/v1/movie/603":
			w.Write([]byte(`{"genres":[{"id":28,"name":"Action"}],"productionCompanies":[{"id":79,"name":"Village Roadshow"}]}`))
		case "/api/v1/user":
			w.Write([]byte(`{"pageInfo":{"pages":1},"results":[{"id":1,"email":"a@b.c","requestCount":3}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, err := NewKeyAuth(srv.URL+"/", "en", "secret")
	if err != nil {
		t.Fatalf("NewKeyAuth: %v", err)
	}

	requests, page, err := c.GetRequests(1, 50, RequestFilterAll, RequestSortAdded)
	if err != nil {
		t.Fatalf("GetRequests: %v", err)
	}
	if page.Pages != 2 || len(requests) != 1 {
		t.Fatalf("pages = %d, requests = %d", page.Pages, len(requests))
	}
	req := requests[0]
	if req.Status.ToString() != "Approved" || req.Media.Status.ToString() != "Available" || !req.IsUHD {
		t.Errorf("unexpected request decode: %+v", req)
	}

	movie, err := req.GetMovieDetails(c)
	if err != nil {
		t.Fatalf("GetMovieDetails: %v", err)
	}
	if movie.Genres[0].Name != "Action" || movie.ProductionCompanies[0].Name != "Village Roadshow" {
		t.Errorf("unexpected movie decode: %+v", movie)
	}
	if _, err := req.GetTVDetails(c); err == nil {
		t.Error("GetTVDetails on movie request should error")
	}

	users, _, err := c.GetAllUsers(50, 0)
	if err != nil {
		t.Fatalf("GetAllUsers: %v", err)
	}
	if users[0].RequestCount != 3 {
		t.Errorf("unexpected user decode: %+v", users[0])
	}

	if _, err := NewKeyAuth(srv.URL, "en", "wrong"); err == nil {
		t.Error("NewKeyAuth with bad key should error")
	}
}
