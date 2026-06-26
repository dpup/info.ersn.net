// Package caloes provides a client for the Cal OES California Evacuation
// Aggregation Layer (ArcGIS, public, keyless). It is an ACTIVE-EVENTS-ONLY layer:
// it holds only zones currently in Order/Warning/Advisory, so an empty result is
// ambiguous (no-evacuations vs feed-broken). Callers MUST treat empty as
// "unknown", never as "all clear" (see docs/hazard-aggregation-design.md §6.4).
package caloes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxBody = 16 << 20 // 16 MiB (zone polygons)

// HTTPDoer interface for HTTP clients (for testability).
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client queries the Cal OES evacuation aggregation feature service.
type Client struct {
	httpClient HTTPDoer
	baseURL    string
}

// NewClient creates a Cal OES client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		baseURL:    "https://services.arcgis.com/BLN4oKB0N1YSgvY8/arcgis/rest/services/CA_EVACUATIONS_CalOESHosted_view/FeatureServer/0/query",
	}
}

// NewClientWithHTTPDoer creates a client with a custom doer + query URL (testing).
func NewClientWithHTTPDoer(queryURL string, httpClient HTTPDoer) *Client {
	return &Client{httpClient: httpClient, baseURL: queryURL}
}

// EvacZone is a normalized active evacuation zone. GeometryType/GeometryCoords
// carry the upstream GeoJSON geometry verbatim.
type EvacZone struct {
	ZoneID         string
	ZoneName       string
	County         string
	Status         string // raw upstream status text
	EventType      string
	PublicInfo     string
	LastUpdated    time.Time
	GeometryType   string
	GeometryCoords json.RawMessage
}

// SourceURL is the authoritative public viewer, always surfaced to users.
const SourceURL = "https://protect.genasys.com/"

// GetActiveEvacuations returns active evacuation zones for the given counties.
// An empty (non-error) result is ambiguous — the caller must treat it as
// "unknown", not "no evacuations".
func (c *Client) GetActiveEvacuations(ctx context.Context, counties []string) ([]EvacZone, error) {
	params := url.Values{}
	params.Set("f", "geojson")
	params.Set("where", countyWhere(counties))
	params.Set("outFields", "ZONE_ID,ZONE_NAME,COUNTY,STATUS,EVENT_TYPE,PUBLIC_INFO,STATEWIDE_LAST_UPDATED")
	params.Set("returnGeometry", "true")
	params.Set("outSR", "4326")

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Cal OES request: %w", err)
	}
	req.Header.Set("Accept", "application/geo+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Cal OES request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("Cal OES API error %d: %s", resp.StatusCode, string(body))
	}

	var parsed evacResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode Cal OES response: %w", err)
	}
	out := make([]EvacZone, 0, len(parsed.Features))
	for _, f := range parsed.Features {
		out = append(out, EvacZone{
			ZoneID:         f.Properties.ZoneID,
			ZoneName:       f.Properties.ZoneName,
			County:         f.Properties.County,
			Status:         f.Properties.Status,
			EventType:      f.Properties.EventType,
			PublicInfo:     f.Properties.PublicInfo,
			LastUpdated:    msToTime(f.Properties.LastUpdated),
			GeometryType:   f.Geometry.Type,
			GeometryCoords: f.Geometry.Coordinates,
		})
	}
	return out, nil
}

func countyWhere(counties []string) string {
	if len(counties) == 0 {
		return "1=1"
	}
	quoted := make([]string, 0, len(counties))
	for _, c := range counties {
		// ArcGIS SQL: escape single quotes.
		quoted = append(quoted, "'"+strings.ReplaceAll(c, "'", "''")+"'")
	}
	return "COUNTY IN (" + strings.Join(quoted, ",") + ")"
}

func msToTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

type evacResponse struct {
	Features []evacFeature `json:"features"`
}

type evacFeature struct {
	Properties evacProps    `json:"properties"`
	Geometry   geometryJSON `json:"geometry"`
}

type evacProps struct {
	ZoneID      string `json:"ZONE_ID"`
	ZoneName    string `json:"ZONE_NAME"`
	County      string `json:"COUNTY"`
	Status      string `json:"STATUS"`
	EventType   string `json:"EVENT_TYPE"`
	PublicInfo  string `json:"PUBLIC_INFO"`
	LastUpdated int64  `json:"STATEWIDE_LAST_UPDATED"`
}

type geometryJSON struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}
