// Package wfigs provides a client for the NIFC WFIGS interagency fire-perimeter
// ArcGIS feature service (public, keyless, CORS-enabled, GeoJSON). We proxy it
// for caching + a consistent shape, and to simplify geometry server-side.
package wfigs

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

const maxBody = 16 << 20 // 16 MiB (simplified polygons; bbox-scoped)

// HTTPDoer interface for HTTP clients (for testability).
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client queries the WFIGS current-perimeters feature service.
type Client struct {
	httpClient HTTPDoer
	baseURL    string
}

// NewClient creates a WFIGS client pointed at the public feature service.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 25 * time.Second},
		baseURL:    "https://services3.arcgis.com/T4QMspbfLg3qTGWY/arcgis/rest/services/WFIGS_Interagency_Perimeters_Current/FeatureServer/0/query",
	}
}

// NewClientWithHTTPDoer creates a client with a custom doer + query URL (testing).
func NewClientWithHTTPDoer(queryURL string, httpClient HTTPDoer) *Client {
	return &Client{httpClient: httpClient, baseURL: queryURL}
}

// Bounds is a lat/lng bounding box.
type Bounds struct {
	MinLatitude, MaxLatitude, MinLongitude, MaxLongitude float64
}

// Perimeter is a normalized fire perimeter. GeometryType/GeometryCoords carry
// the upstream GeoJSON geometry verbatim ([lon,lat] order, already simplified
// server-side); the caller wraps them into its own geometry type.
type Perimeter struct {
	Name             string
	Acres            float64
	PercentContained int32
	Cause            string
	GeometryType     string
	GeometryCoords   json.RawMessage
}

// GetPerimeters returns perimeters intersecting bounds, geometry simplified
// server-side (maxAllowableOffset).
func (c *Client) GetPerimeters(ctx context.Context, b Bounds) ([]Perimeter, error) {
	params := url.Values{}
	params.Set("f", "geojson")
	params.Set("where", "1=1")
	params.Set("geometry", fmt.Sprintf("%s,%s,%s,%s",
		ftoa(b.MinLongitude), ftoa(b.MinLatitude), ftoa(b.MaxLongitude), ftoa(b.MaxLatitude)))
	params.Set("geometryType", "esriGeometryEnvelope")
	params.Set("inSR", "4326")
	params.Set("spatialRel", "esriSpatialRelIntersects")
	params.Set("outFields", "poly_IncidentName,attr_IncidentSize,attr_PercentContained,attr_FireCause")
	params.Set("returnGeometry", "true")
	params.Set("maxAllowableOffset", "0.001") // ~100m vertex tolerance

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create WFIGS request: %w", err)
	}
	req.Header.Set("Accept", "application/geo+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute WFIGS request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("WFIGS API error %d: %s", resp.StatusCode, string(body))
	}

	var parsed perimeterResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode WFIGS response: %w", err)
	}
	out := make([]Perimeter, 0, len(parsed.Features))
	for _, f := range parsed.Features {
		out = append(out, Perimeter{
			Name:             f.Properties.Name,
			Acres:            f.Properties.Acres,
			PercentContained: int32(f.Properties.PercentContained),
			Cause:            f.Properties.Cause,
			GeometryType:     f.Geometry.Type,
			GeometryCoords:   f.Geometry.Coordinates,
		})
	}
	return out, nil
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

type perimeterResponse struct {
	Features []perimeterFeature `json:"features"`
}

type perimeterFeature struct {
	Properties perimeterProps `json:"properties"`
	Geometry   geometryJSON   `json:"geometry"`
}

type perimeterProps struct {
	Name             string  `json:"poly_IncidentName"`
	Acres            float64 `json:"attr_IncidentSize"`
	PercentContained float64 `json:"attr_PercentContained"`
	Cause            string  `json:"attr_FireCause"`
}

type geometryJSON struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}
