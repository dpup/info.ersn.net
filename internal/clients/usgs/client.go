// Package usgs provides a client for the USGS earthquake catalog (FDSN event
// API). Public, no API key, CORS-enabled. We proxy it anyway for one consistent
// shape + caching alongside the other hazard layers.
package usgs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// maxBody caps the upstream response (defensive; a bbox query is small).
const maxBody = 5 << 20 // 5 MiB

// HTTPDoer interface for HTTP clients (for testability).
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client queries the USGS FDSN event service.
type Client struct {
	httpClient HTTPDoer
	baseURL    string
}

// NewClient creates a USGS client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		baseURL:    "https://earthquake.usgs.gov",
	}
}

// NewClientWithHTTPDoer creates a client with a custom doer + base URL (testing).
func NewClientWithHTTPDoer(baseURL string, httpClient HTTPDoer) *Client {
	return &Client{httpClient: httpClient, baseURL: baseURL}
}

// Quake is a normalized earthquake event.
type Quake struct {
	ID        string
	Magnitude float64
	DepthKm   float64
	Place     string
	Time      time.Time
	Updated   time.Time
	Felt      int32 // DYFI report count (0 if none)
	URL       string
	Lat, Lng  float64
}

// Bounds is a lat/lng bounding box.
type Bounds struct {
	MinLatitude, MaxLatitude, MinLongitude, MaxLongitude float64
}

// GetEarthquakes returns events within bounds at or above minMag in the last
// `within` duration, newest first.
func (c *Client) GetEarthquakes(ctx context.Context, b Bounds, minMag float64, within time.Duration) ([]Quake, error) {
	params := url.Values{}
	params.Set("format", "geojson")
	params.Set("orderby", "time")
	params.Set("minmagnitude", strconv.FormatFloat(minMag, 'f', -1, 64))
	params.Set("minlatitude", ftoa(b.MinLatitude))
	params.Set("maxlatitude", ftoa(b.MaxLatitude))
	params.Set("minlongitude", ftoa(b.MinLongitude))
	params.Set("maxlongitude", ftoa(b.MaxLongitude))
	params.Set("starttime", time.Now().Add(-within).UTC().Format("2006-01-02T15:04:05"))

	requestURL := fmt.Sprintf("%s/fdsnws/event/1/query?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create USGS request: %w", err)
	}
	req.Header.Set("Accept", "application/geo+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute USGS request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("USGS API error %d: %s", resp.StatusCode, string(body))
	}

	var parsed quakeResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode USGS response: %w", err)
	}
	return parsed.toQuakes(), nil
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// GeoJSON response (only the fields we use).
type quakeResponse struct {
	Features []quakeFeature `json:"features"`
}

type quakeFeature struct {
	ID         string          `json:"id"`
	Properties quakeProperties `json:"properties"`
	Geometry   quakeGeometry   `json:"geometry"`
}

type quakeProperties struct {
	Mag     *float64 `json:"mag"`
	Place   string   `json:"place"`
	Time    int64    `json:"time"`    // ms epoch
	Updated int64    `json:"updated"` // ms epoch
	Felt    *int32   `json:"felt"`
	URL     string   `json:"url"`
}

type quakeGeometry struct {
	Coordinates []float64 `json:"coordinates"` // [lon, lat, depthKm]
}

func (r quakeResponse) toQuakes() []Quake {
	var out []Quake
	for _, f := range r.Features {
		q := Quake{
			ID:    f.ID,
			Place: f.Properties.Place,
			URL:   f.Properties.URL,
		}
		if f.Properties.Mag != nil {
			q.Magnitude = *f.Properties.Mag
		}
		if f.Properties.Felt != nil {
			q.Felt = *f.Properties.Felt
		}
		if f.Properties.Time > 0 {
			q.Time = time.UnixMilli(f.Properties.Time).UTC()
		}
		if f.Properties.Updated > 0 {
			q.Updated = time.UnixMilli(f.Properties.Updated).UTC()
		}
		if len(f.Geometry.Coordinates) >= 3 {
			q.Lng = f.Geometry.Coordinates[0]
			q.Lat = f.Geometry.Coordinates[1]
			q.DepthKm = f.Geometry.Coordinates[2]
		}
		out = append(out, q)
	}
	return out
}
