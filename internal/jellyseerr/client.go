// Package jellyseerr is a minimal client for the subset of the Jellyseerr v1
// API this exporter uses. It replaces github.com/willfantom/goverseerr, which
// has been unmaintained since 2021.
package jellyseerr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type RequestStatus int

const (
	RequestStatusPending   RequestStatus = 1
	RequestStatusApproved  RequestStatus = 2
	RequestStatusDeclined  RequestStatus = 3
	RequestStatusAvailable RequestStatus = 4
)

func (s RequestStatus) ToString() string {
	switch s {
	case RequestStatusApproved:
		return "Approved"
	case RequestStatusDeclined:
		return "Declined"
	case RequestStatusAvailable:
		return "Available"
	case RequestStatusPending:
		return "Pending"
	default:
		return "Unknown"
	}
}

type MediaStatus int

const (
	MediaStatusUnknown    MediaStatus = 0
	MediaStatusPending    MediaStatus = 1
	MediaStatusProcessing MediaStatus = 3
	MediaStatusPartial    MediaStatus = 4
	MediaStatusAvailable  MediaStatus = 5
)

func (s MediaStatus) ToString() string {
	switch s {
	case MediaStatusAvailable:
		return "Available"
	case MediaStatusPartial:
		return "Part-Available"
	case MediaStatusProcessing:
		return "Processing"
	case MediaStatusPending:
		return "Pending"
	default:
		return "Unknown"
	}
}

type MediaType string

const (
	MediaTypeTV    MediaType = "tv"
	MediaTypeMovie MediaType = "movie"
)

type (
	RequestFilter string
	RequestSort   string
)

const (
	RequestFilterAll RequestFilter = "all"
	RequestSortAdded RequestSort   = "added"
)

type Page struct {
	Page    int `json:"page"`
	Pages   int `json:"pages"`
	Results int `json:"results"`
}

type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	RequestCount int       `json:"requestCount"`
	Created      time.Time `json:"createdAt"`
}

type MediaInfo struct {
	ID        int         `json:"id"`
	TMDB      int         `json:"tmdbId"`
	MediaType MediaType   `json:"mediaType"`
	Status    MediaStatus `json:"status"`
}

type MediaRequest struct {
	ID     int           `json:"id"`
	Status RequestStatus `json:"status"`
	Media  MediaInfo     `json:"media"`
	IsUHD  bool          `json:"is4k"`
}

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ProductionCompany struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Network struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type MovieDetails struct {
	Genres              []Genre             `json:"genres"`
	ProductionCompanies []ProductionCompany `json:"productionCompanies"`
}

type TVDetails struct {
	Genres   []Genre   `json:"genres"`
	Networks []Network `json:"networks"`
}

// Client talks to a Jellyseerr instance using API key auth.
type Client struct {
	baseURL string
	locale  string
	apiKey  string
	http    *http.Client
}

// NewKeyAuth creates a Client and verifies the API key against /auth/me.
func NewKeyAuth(rawURL, locale, apiKey string) (*Client, error) {
	c := &Client{
		baseURL: strings.TrimSuffix(rawURL, "/") + "/api/v1",
		locale:  locale,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	var me User
	if err := c.get("/auth/me", nil, &me); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) get(path string, query url.Values, out any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 status code (%d)", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// GetRequests returns one page of media requests.
func (c *Client) GetRequests(pageNumber, pageSize int, filter RequestFilter, sort RequestSort) ([]*MediaRequest, *Page, error) {
	var response struct {
		PageInfo Page            `json:"pageInfo"`
		Results  []*MediaRequest `json:"results"`
	}
	query := url.Values{
		"take":   {strconv.Itoa(pageSize)},
		"skip":   {strconv.Itoa(pageSize * pageNumber)},
		"filter": {string(filter)},
		"sort":   {string(sort)},
	}
	if err := c.get("/request", query, &response); err != nil {
		return nil, nil, err
	}
	return response.Results, &response.PageInfo, nil
}

// GetAllUsers returns one page of users.
func (c *Client) GetAllUsers(pageSize, pageNumber int) ([]*User, *Page, error) {
	var response struct {
		PageInfo Page    `json:"pageInfo"`
		Results  []*User `json:"results"`
	}
	query := url.Values{
		"take": {strconv.Itoa(pageSize)},
		"skip": {strconv.Itoa(pageSize * pageNumber)},
	}
	if err := c.get("/user", query, &response); err != nil {
		return nil, nil, err
	}
	return response.Results, &response.PageInfo, nil
}

// GetMovieDetails fetches TMDB movie details for a movie request.
func (req MediaRequest) GetMovieDetails(c *Client) (*MovieDetails, error) {
	if req.Media.MediaType != MediaTypeMovie {
		return nil, fmt.Errorf("request's media type is not movie")
	}
	var details MovieDetails
	err := c.get("/movie/"+strconv.Itoa(req.Media.TMDB), url.Values{"language": {c.locale}}, &details)
	if err != nil {
		return nil, err
	}
	return &details, nil
}

// GetTVDetails fetches TMDB TV details for a TV request.
func (req MediaRequest) GetTVDetails(c *Client) (*TVDetails, error) {
	if req.Media.MediaType != MediaTypeTV {
		return nil, fmt.Errorf("request's media type is not tv")
	}
	var details TVDetails
	err := c.get("/tv/"+strconv.Itoa(req.Media.TMDB), url.Values{"language": {c.locale}}, &details)
	if err != nil {
		return nil, err
	}
	return &details, nil
}
