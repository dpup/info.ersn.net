package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	api "github.com/dpup/info.ersn.net/server"
)

// Client provides access to Google Routes API v2
// Implementation per research.md lines 32-47
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// RouteData represents the processed route information from Google Routes API
type RouteData struct {
	DurationSeconds int32
	DistanceMeters  int32
	Polyline        string
	SpeedReadings   []SpeedReading
}

// SpeedReading represents traffic speed data for route segments
type SpeedReading struct {
	StartIndex    int32
	EndIndex      int32
	SpeedCategory string // "NORMAL", "SLOW", "TRAFFIC_JAM"
}

// NewClient creates a new Google Routes API client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: "https://routes.googleapis.com",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ComputeRoutes performs coordinate-based route computation
// Implements field mask requirements from research.md line 44
func (c *Client) ComputeRoutes(ctx context.Context, origin, destination *api.Coordinates) (*RouteData, error) {
	// Build request body per research.md lines 45-53
	requestBody := map[string]interface{}{
		"origin": map[string]interface{}{
			"location": map[string]interface{}{
				"latLng": map[string]interface{}{
					"latitude":  origin.Latitude,
					"longitude": origin.Longitude,
				},
			},
		},
		"destination": map[string]interface{}{
			"location": map[string]interface{}{
				"latLng": map[string]interface{}{
					"latitude":  destination.Latitude,
					"longitude": destination.Longitude,
				},
			},
		},
		"travelMode":         "DRIVE",
		"routingPreference":  "TRAFFIC_AWARE",
		"extraComputations":  []string{"TRAFFIC_ON_POLYLINE"},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request with mandatory headers per research.md line 42-44
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/directions/v2:computeRoutes", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Critical: Field mask is REQUIRED or API returns errors (research.md line 44)
	req.Header.Set("X-Goog-Api-Key", c.apiKey)
	req.Header.Set("X-Goog-FieldMask", "routes.duration,routes.distanceMeters,routes.polyline.encodedPolyline,routes.travelAdvisory.speedReadingIntervals")
	req.Header.Set("Content-Type", "application/json")

	// Execute request with rate limiting awareness (3K QPM from research.md line 56)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Handle rate limiting and errors per research.md line 57
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limit exceeded (3K QPM)")
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response GoogleRoutesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Routes) == 0 {
		return nil, fmt.Errorf("no routes found in response")
	}

	return c.processRouteResponse(response.Routes[0])
}

// processRouteResponse converts Google Routes API response to our RouteData format
func (c *Client) processRouteResponse(route GoogleRoute) (*RouteData, error) {
	// Parse duration from string format like "450s"
	durationSeconds, err := parseDuration(route.Duration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse duration: %w", err)
	}

	// Process speed readings from traffic advisory
	var speedReadings []SpeedReading
	if route.TravelAdvisory != nil && len(route.TravelAdvisory.SpeedReadingIntervals) > 0 {
		for _, interval := range route.TravelAdvisory.SpeedReadingIntervals {
			speedReadings = append(speedReadings, SpeedReading{
				StartIndex:    interval.StartPolylinePointIndex,
				EndIndex:      interval.EndPolylinePointIndex,
				SpeedCategory: interval.Speed,
			})
		}
	}

	return &RouteData{
		DurationSeconds: durationSeconds,
		DistanceMeters:  route.DistanceMeters,
		Polyline:        route.Polyline.EncodedPolyline,
		SpeedReadings:   speedReadings,
	}, nil
}

// parseDuration parses Google's duration format like "450s" to seconds
func parseDuration(durationStr string) (int32, error) {
	if durationStr == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// Simple parser for "Ns" format
	if len(durationStr) > 1 && durationStr[len(durationStr)-1] == 's' {
		durationStr = durationStr[:len(durationStr)-1]
	}

	var seconds int32
	_, err := fmt.Sscanf(durationStr, "%d", &seconds)
	return seconds, err
}

// GoogleRoutesResponse represents the API response structure
type GoogleRoutesResponse struct {
	Routes []GoogleRoute `json:"routes"`
}

// GoogleRoute represents a single route in the response
type GoogleRoute struct {
	Duration        string             `json:"duration"`
	DistanceMeters  int32              `json:"distanceMeters"`
	Polyline        GooglePolyline     `json:"polyline"`
	TravelAdvisory  *GoogleTravelAdvisory `json:"travelAdvisory,omitempty"`
}

// GooglePolyline represents the route polyline
type GooglePolyline struct {
	EncodedPolyline string `json:"encodedPolyline"`
}

// GoogleTravelAdvisory represents traffic information
type GoogleTravelAdvisory struct {
	SpeedReadingIntervals []GoogleSpeedInterval `json:"speedReadingIntervals"`
}

// GoogleSpeedInterval represents speed data for a route segment
type GoogleSpeedInterval struct {
	StartPolylinePointIndex int32  `json:"startPolylinePointIndex"`
	EndPolylinePointIndex   int32  `json:"endPolylinePointIndex"`
	Speed                   string `json:"speed"` // "NORMAL", "SLOW", "TRAFFIC_JAM"
}