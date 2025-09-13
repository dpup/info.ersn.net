package caltrans

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/dpup/info.ersn.net/server/internal/lib/geo"
)


// mockHTTPClient provides local KML file responses for testing
type mockHTTPClient struct {
	testDataDir string
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	var filename string
	switch req.URL.String() {
	case "https://quickmap.dot.ca.gov/data/lcs2way.kml":
		filename = "lane_closures.kml"
	case "https://quickmap.dot.ca.gov/data/chp-only.kml":
		filename = "chp_incidents.kml"
	case "https://quickmap.dot.ca.gov/data/cc.kml":
		filename = "chain_controls.kml"
	default:
		return &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(strings.NewReader("Not found")),
		}, nil
	}

	filePath := filepath.Join(m.testDataDir, filename)
	file, err := os.Open(filePath)
	if err != nil {
		return &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader("Internal server error")),
		}, err
	}

	return &http.Response{
		StatusCode: 200,
		Body:       file,
	}, nil
}

func setupTestParser(t *testing.T) *FeedParser {
	testDataDir := filepath.Join("..", "..", "..", "tests", "testdata", "caltrans")
	
	// Verify test data exists
	_, err := os.Stat(testDataDir)
	require.NoError(t, err, "Test data directory not found. Run 'make test-setup' to download test data.")

	parser := &FeedParser{
		HTTPClient: &mockHTTPClient{testDataDir: testDataDir},
		geoUtils:   geo.NewGeoUtils(),
	}
	
	return parser
}

func TestParseLaneClosures(t *testing.T) {
	parser := setupTestParser(t)
	
	incidents, err := parser.ParseLaneClosures(context.Background())
	
	require.NoError(t, err)
	assert.Greater(t, len(incidents), 0, "Should parse some lane closure incidents")
	
	// Verify structure of first incident
	if len(incidents) > 0 {
		incident := incidents[0]
		assert.Equal(t, LANE_CLOSURE, incident.FeedType)
		assert.NotEmpty(t, incident.Name)
		assert.NotNil(t, incident.Coordinates)
		assert.NotZero(t, incident.LastFetched)
	}
}

func TestParseCHPIncidents(t *testing.T) {
	parser := setupTestParser(t)
	
	incidents, err := parser.ParseCHPIncidents(context.Background())
	
	require.NoError(t, err)
	assert.Greater(t, len(incidents), 0, "Should parse some CHP incidents")
	
	// Verify structure of first incident
	if len(incidents) > 0 {
		incident := incidents[0]
		assert.Equal(t, CHP_INCIDENT, incident.FeedType)
		assert.NotEmpty(t, incident.Name)
		assert.NotNil(t, incident.Coordinates)
		assert.NotZero(t, incident.LastFetched)
	}
}

func TestParseChainControls(t *testing.T) {
	parser := setupTestParser(t)
	
	incidents, err := parser.ParseChainControls(context.Background())
	
	require.NoError(t, err)
	// Chain controls may be empty in summer, so just verify it doesn't error
	
	// If we have incidents, verify structure
	if len(incidents) > 0 {
		incident := incidents[0]
		assert.Equal(t, CHAIN_CONTROL, incident.FeedType)
		assert.NotEmpty(t, incident.Name)
		assert.NotNil(t, incident.Coordinates)
		assert.NotZero(t, incident.LastFetched)
	}
}

func TestParseWithGeographicFilter(t *testing.T) {
	parser := setupTestParser(t)
	
	// Test with San Francisco coordinates (should have incidents nearby)
	routeCoordinates := []geo.Point{
		{Latitude: 37.7749, Longitude: -122.4194}, // Downtown San Francisco
	}
	
	t.Run("10km radius", func(t *testing.T) {
		incidents, err := parser.ParseWithGeographicFilter(context.Background(), routeCoordinates, 10000)
		require.NoError(t, err)
		
		// Should find some incidents within 10km of SF
		assert.Greater(t, len(incidents), 0, "Should find incidents within 10km of San Francisco")
		
		// Verify all incidents are within the specified radius
		geoUtils := geo.NewGeoUtils()
		for _, incident := range incidents {
			distance, err := geoUtils.DistanceFromCoords(
				37.7749, -122.4194,
				incident.Coordinates.Latitude, incident.Coordinates.Longitude,
			)
			require.NoError(t, err)
			assert.LessOrEqual(t, distance, 10000.0, "All incidents should be within 10km radius")
		}
	})
	
	t.Run("1km radius", func(t *testing.T) {
		incidents, err := parser.ParseWithGeographicFilter(context.Background(), routeCoordinates, 1000)
		require.NoError(t, err)
		
		// Should find fewer incidents within 1km
		// Verify all incidents are within the specified radius
		geoUtils := geo.NewGeoUtils()
		for _, incident := range incidents {
			distance, err := geoUtils.DistanceFromCoords(
				37.7749, -122.4194,
				incident.Coordinates.Latitude, incident.Coordinates.Longitude,
			)
			require.NoError(t, err)
			assert.LessOrEqual(t, distance, 1000.0, "All incidents should be within 1km radius")
		}
	})
}

func TestHaversineDistance(t *testing.T) {
	tests := []struct {
		name     string
		lat1     float64
		lon1     float64
		lat2     float64
		lon2     float64
		expected float64
		delta    float64
	}{
		{
			name:     "San Francisco to Los Angeles",
			lat1:     37.7749,
			lon1:     -122.4194,
			lat2:     34.0522,
			lon2:     -118.2437,
			expected: 559120, // approximately 559km
			delta:    5000,   // 5km tolerance
		},
		{
			name:     "Same point",
			lat1:     37.7749,
			lon1:     -122.4194,
			lat2:     37.7749,
			lon2:     -122.4194,
			expected: 0,
			delta:    1,
		},
		{
			name:     "Short distance in SF",
			lat1:     37.7749,  // Downtown SF
			lon1:     -122.4194,
			lat2:     37.8044,  // North Beach
			lon2:     -122.4078,
			expected: 3435,     // approximately 3.4km (corrected)
			delta:    200,      // 200m tolerance
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			geoUtils := geo.NewGeoUtils()
			result, err := geoUtils.DistanceFromCoords(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			require.NoError(t, err)
			assert.InDelta(t, tt.expected, result, tt.delta, 
				"Distance should be approximately %v meters", tt.expected)
		})
	}
}

func TestExtractTextFromHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic HTML removal",
			input:    "<p>Hello <b>world</b>!</p>",
			expected: "Hello world !",
		},
		{
			name:     "HTML entities",
			input:    "Route &amp; Highway",
			expected: "Route & Highway",
		},
		{
			name:     "Multiple whitespace cleanup",
			input:    "<div>  Multiple   \n  spaces  </div>",
			expected: "Multiple spaces",
		},
		{
			name:     "Empty input",
			input:    "",
			expected: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTextFromHTML(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Closed status",
			input:    "Highway 4 is CLOSED due to snow",
			expected: "closed",
		},
		{
			name:     "Chain control",
			input:    "Chain control in effect from mile marker 10",
			expected: "chain control in effect",
		},
		{
			name:     "Construction",
			input:    "Road construction project ongoing",
			expected: "construction",
		},
		{
			name:     "No status",
			input:    "Normal traffic conditions",
			expected: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractStatus(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractDates(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Single date MM/DD/YYYY",
			input:    "Expected to end at 5:00pm 12/25/2024",
			expected: []string{"12/25/2024"},
		},
		{
			name:     "Date with text format",
			input:    "Starting Dec 15, 2024 until further notice",
			expected: []string{"Dec 15, 2024"},
		},
		{
			name:     "Multiple dates",
			input:    "From 01/01/2025 to 12/31/2025",
			expected: []string{"01/01/2025", "12/31/2025"},
		},
		{
			name:     "No dates",
			input:    "No specific dates mentioned",
			expected: []string{},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDates(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractGeometry(t *testing.T) {
	parser := NewFeedParser()

	t.Run("Point geometry", func(t *testing.T) {
		placemark := &Placemark{
			Point: Point{
				Coordinates: "-120.5000,38.1000,0",
			},
		}

		coord, polyline := parser.extractGeometry(placemark)
		
		require.NotNil(t, coord)
		assert.Equal(t, 38.1000, coord.Latitude)
		assert.Equal(t, -120.5000, coord.Longitude)
		assert.Nil(t, polyline)
	})

	t.Run("LineString geometry", func(t *testing.T) {
		placemark := &Placemark{
			LineString: LineString{
				Coordinates: "-120.5000,38.1000,0 -120.4500,38.1200,0 -120.4000,38.1400,0",
			},
		}

		coord, polyline := parser.extractGeometry(placemark)
		
		require.NotNil(t, coord)
		require.NotNil(t, polyline)
		assert.Equal(t, 38.1000, coord.Latitude) // First point
		assert.Equal(t, 3, len(polyline.Points))
		assert.Equal(t, 38.1400, polyline.Points[2].Latitude) // Last point
	})

	t.Run("Polygon geometry", func(t *testing.T) {
		placemark := &Placemark{
			Polygon: Polygon{
				OuterBoundary: OuterBoundary{
					LinearRing: LinearRing{
						Coordinates: "-120.5000,38.1000,0 -120.4500,38.1200,0 -120.4000,38.1400,0 -120.5000,38.1000,0",
					},
				},
			},
		}

		coord, polyline := parser.extractGeometry(placemark)
		
		require.NotNil(t, coord)
		require.NotNil(t, polyline)
		assert.Equal(t, 4, len(polyline.Points)) // Polygon with closing point
	})

	t.Run("MultiGeometry", func(t *testing.T) {
		placemark := &Placemark{
			MultiGeometry: MultiGeometry{
				Points: []Point{
					{Coordinates: "-120.5000,38.1000,0"},
				},
				LineStrings: []LineString{
					{Coordinates: "-120.4500,38.1200,0 -120.4000,38.1400,0"},
				},
			},
		}

		coord, polyline := parser.extractGeometry(placemark)
		
		require.NotNil(t, coord)
		require.NotNil(t, polyline)
		assert.Equal(t, 3, len(polyline.Points)) // 1 point + 2 linestring points
	})

	t.Run("No geometry", func(t *testing.T) {
		placemark := &Placemark{
			Name: "Test placemark with no geometry",
		}

		coord, polyline := parser.extractGeometry(placemark)
		
		assert.Nil(t, coord)
		assert.Nil(t, polyline)
	})
}

func TestParseCoordinateList(t *testing.T) {
	parser := NewFeedParser()

	t.Run("Multiple coordinates", func(t *testing.T) {
		coordString := "-120.5000,38.1000,0 -120.4500,38.1200,0 -120.4000,38.1400,0"
		coords := parser.parseCoordinateList(coordString)
		
		require.Equal(t, 3, len(coords))
		assert.Equal(t, 38.1000, coords[0].Latitude)
		assert.Equal(t, -120.5000, coords[0].Longitude)
		assert.Equal(t, 38.1400, coords[2].Latitude)
		assert.Equal(t, -120.4000, coords[2].Longitude)
	})

	t.Run("Single coordinate", func(t *testing.T) {
		coordString := "-120.5000,38.1000,0"
		coords := parser.parseCoordinateList(coordString)
		
		require.Equal(t, 1, len(coords))
		assert.Equal(t, 38.1000, coords[0].Latitude)
		assert.Equal(t, -120.5000, coords[0].Longitude)
	})

	t.Run("Empty coordinate string", func(t *testing.T) {
		coords := parser.parseCoordinateList("")
		assert.Nil(t, coords)
	})

	t.Run("Invalid coordinates", func(t *testing.T) {
		coordString := "invalid,coordinate,data"
		coords := parser.parseCoordinateList(coordString)
		assert.Empty(t, coords)
	})
}

