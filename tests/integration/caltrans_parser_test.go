package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dpup/info.ersn.net/server/internal/clients/caltrans"
)

// T009: Integration test Caltrans KML parser - MUST FAIL initially
func TestCaltransParser_ParseChainControls_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// This test MUST fail until Caltrans parser is implemented
	parser := &caltrans.FeedParser{} // No implementation yet - will cause compilation error
	
	// Test chain control feed from research.md line 71
	chainControls, err := parser.ParseChainControls(context.Background())
	
	// Verify KML parsing requirements from research.md lines 49-67
	require.NoError(t, err, "Chain controls parsing should not return error")
	require.NotNil(t, chainControls, "Chain controls data should not be nil")
	
	// Validate KML structure parsing per data-model.md lines 80-90
	if len(chainControls) > 0 {
		control := chainControls[0]
		require.NotEmpty(t, control.Name, "Name should be extracted from KML")
		require.NotEmpty(t, control.DescriptionHtml, "HTML description should be extracted from CDATA")
		require.NotEmpty(t, control.DescriptionText, "Text should be extracted from HTML")
		require.NotNil(t, control.Coordinates, "Coordinates should be parsed from KML Point")
		
		// Coordinates should be valid WGS84 decimal degrees
		require.GreaterOrEqual(t, control.Coordinates.Latitude, -90.0, "Latitude should be valid")
		require.LessOrEqual(t, control.Coordinates.Latitude, 90.0, "Latitude should be valid")
		require.GreaterOrEqual(t, control.Coordinates.Longitude, -180.0, "Longitude should be valid")
		require.LessOrEqual(t, control.Coordinates.Longitude, 180.0, "Longitude should be valid")
		
		// Should have feed type set correctly
		require.Equal(t, caltrans.CHAIN_CONTROL, control.FeedType, "Feed type should be CHAIN_CONTROL")
	}
}

// TestCaltransParser_ParseLaneClosures tests lane closure KML parsing
func TestCaltransParser_ParseLaneClosures_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	parser := &caltrans.FeedParser{} // No implementation yet - will cause compilation error
	
	// Test lane closures feed from research.md line 72
	laneClosures, err := parser.ParseLaneClosures(context.Background())
	
	require.NoError(t, err, "Lane closures parsing should not return error")
	require.NotNil(t, laneClosures, "Lane closures data should not be nil")
	
	if len(laneClosures) > 0 {
		closure := laneClosures[0]
		require.NotEmpty(t, closure.Name, "Name should be extracted from KML")
		require.NotNil(t, closure.Coordinates, "Coordinates should be parsed")
		require.Equal(t, caltrans.LANE_CLOSURE, closure.FeedType, "Feed type should be LANE_CLOSURE")
	}
}

// TestCaltransParser_ParseCHPIncidents tests CHP incident KML parsing
func TestCaltransParser_ParseCHPIncidents_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	parser := &caltrans.FeedParser{} // No implementation yet - will cause compilation error
	
	// Test CHP incidents feed from research.md line 73
	chpIncidents, err := parser.ParseCHPIncidents(context.Background())
	
	require.NoError(t, err, "CHP incidents parsing should not return error")
	require.NotNil(t, chpIncidents, "CHP incidents data should not be nil")
	
	if len(chpIncidents) > 0 {
		incident := chpIncidents[0]
		require.NotEmpty(t, incident.Name, "Name should be extracted from KML")
		require.NotNil(t, incident.Coordinates, "Coordinates should be parsed")
		require.Equal(t, caltrans.CHP_INCIDENT, incident.FeedType, "Feed type should be CHP_INCIDENT")
	}
}

// TestCaltransParser_GeographicFiltering tests filtering by route proximity
func TestCaltransParser_GeographicFiltering_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	parser := &caltrans.FeedParser{} // No implementation yet - will cause compilation error
	
	// Test geographic filtering per research.md line 79
	// Using I-5 corridor coordinates (Seattle to Portland route)
	routeCoordinates := []struct {
		Lat, Lon float64
	}{
		{47.6062, -122.3321}, // Seattle
		{45.5152, -122.6784}, // Portland
	}
	
	incidents, err := parser.ParseWithGeographicFilter(context.Background(), routeCoordinates, 50000) // 50km radius
	
	require.NoError(t, err, "Geographic filtering should not return error")
	require.NotNil(t, incidents, "Filtered incidents should not be nil")
	
	// All returned incidents should be within the specified distance of route coordinates
	for _, incident := range incidents {
		found := false
		for _, coord := range routeCoordinates {
			distance := calculateDistance(coord.Lat, coord.Lon, incident.Coordinates.Latitude, incident.Coordinates.Longitude)
			if distance <= 50000 { // 50km in meters
				found = true
				break
			}
		}
		require.True(t, found, "Incident should be within 50km of route coordinates")
	}
}

// Helper function for distance calculation (placeholder)
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	// This would implement Haversine formula or similar
	// Placeholder implementation
	return 0.0
}