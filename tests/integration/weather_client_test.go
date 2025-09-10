package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dpup/info.ersn.net/server/internal/clients/weather"
	api "github.com/dpup/info.ersn.net/server/api/v1"
)

// T010: Integration test OpenWeatherMap client - MUST FAIL initially
func TestWeatherClient_GetCurrentWeather_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// This test MUST fail until OpenWeatherMap client is implemented
	client := &weather.Client{} // No implementation yet - will cause compilation error
	
	// Test coordinates: Seattle (from research.md line 166)
	coordinates := &api.Coordinates{
		Latitude:  47.6062,
		Longitude: -122.3321,
	}
	
	weatherData, err := client.GetCurrentWeather(context.Background(), coordinates)
	
	// Verify API integration requirements from research.md lines 68-82
	require.NoError(t, err, "GetCurrentWeather should not return error with valid coordinates")
	require.NotNil(t, weatherData, "Weather data should not be nil")
	
	// Validate weather data structure matches OpenWeatherMap API response per data-model.md lines 123-146
	require.NotEmpty(t, weatherData.WeatherMain, "Weather main should not be empty")
	require.NotEmpty(t, weatherData.WeatherDescription, "Weather description should not be empty")
	require.NotEmpty(t, weatherData.WeatherIcon, "Weather icon should not be empty")
	
	// Temperature should be reasonable (consistent units: Celsius per research.md decision)
	require.Greater(t, weatherData.TemperatureCelsius, float32(-50), "Temperature should be above absolute minimum")
	require.Less(t, weatherData.TemperatureCelsius, float32(60), "Temperature should be below extreme maximum")
	
	// Humidity should be valid percentage
	require.GreaterOrEqual(t, weatherData.HumidityPercent, int32(0), "Humidity should be >= 0")
	require.LessOrEqual(t, weatherData.HumidityPercent, int32(100), "Humidity should be <= 100")
	
	// Wind direction should be valid degrees
	require.GreaterOrEqual(t, weatherData.WindDirectionDegrees, int32(0), "Wind direction should be >= 0")
	require.LessOrEqual(t, weatherData.WindDirectionDegrees, int32(360), "Wind direction should be <= 360")
	
	// Visibility should be positive
	require.Greater(t, weatherData.VisibilityMeters, int32(0), "Visibility should be positive")
}

// TestWeatherClient_GetWeatherAlerts tests weather alerts from One Call API 3.0
func TestWeatherClient_GetWeatherAlerts_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	client := &weather.Client{} // No implementation yet - will cause compilation error
	
	// Test coordinates: Seattle
	coordinates := &api.Coordinates{
		Latitude:  47.6062,
		Longitude: -122.3321,
	}
	
	alerts, err := client.GetWeatherAlerts(context.Background(), coordinates)
	
	// Verify alerts API integration from research.md line 93
	require.NoError(t, err, "GetWeatherAlerts should not return error")
	require.NotNil(t, alerts, "Alerts array should not be nil (may be empty)")
	
	// Validate alert structure if alerts present
	for _, alert := range alerts {
		require.NotEmpty(t, alert.Id, "Alert ID should not be empty")
		require.NotEmpty(t, alert.SenderName, "Sender name should not be empty")
		require.NotEmpty(t, alert.Event, "Event should not be empty")
		require.Greater(t, alert.StartTimestamp, int64(0), "Start timestamp should be positive")
		require.Greater(t, alert.EndTimestamp, alert.StartTimestamp, "End timestamp should be after start")
		require.NotEmpty(t, alert.Description, "Description should not be empty")
		
		// Tags array should exist (per research.md line 97-98)
		require.NotNil(t, alert.Tags, "Tags array should not be nil")
		
		// Validate OpenWeatherMap alert tags if present (14 categories from data-model.md lines 183-187)
		validTags := []string{
			"extreme temperature value", "fog", "high wind", "thunderstorms", "tornado",
			"hurricane/typhoon", "snow", "ice", "rain", "coastal event", "volcano", "tsunami", "other",
		}
		
		for _, tag := range alert.Tags {
			found := false
			for _, validTag := range validTags {
				if tag == validTag {
					found = true
					break
				}
			}
			// Note: This is informational - OpenWeatherMap may add new tags
			if !found {
				t.Logf("Unknown weather alert tag: %s", tag)
			}
		}
	}
}

// TestWeatherClient_RateLimiting tests the 60/minute rate limit handling
func TestWeatherClient_RateLimiting_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rate limiting test in short mode")
	}
	
	client := &weather.Client{} // No implementation yet - will cause compilation error
	
	coordinates := &api.Coordinates{Latitude: 47.6062, Longitude: -122.3321}
	
	// Single request should succeed within rate limits (60/minute from research.md line 99)
	weatherData, err := client.GetCurrentWeather(context.Background(), coordinates)
	require.NoError(t, err, "Single request should succeed within rate limits")
	require.NotNil(t, weatherData, "Weather data should be returned")
	
	// Note: Actual rate limit testing would require multiple requests
	// This is a placeholder for rate limiting validation
}

// TestWeatherClient_CoordinatePrecision tests coordinate precision handling
func TestWeatherClient_CoordinatePrecision_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	client := &weather.Client{} // No implementation yet - will cause compilation error
	
	// Test high precision coordinates (6+ decimal places per research.md line 96)
	coordinates := &api.Coordinates{
		Latitude:  47.606209, // 6 decimal places
		Longitude: -122.332100,
	}
	
	weatherData, err := client.GetCurrentWeather(context.Background(), coordinates)
	require.NoError(t, err, "Should handle high precision coordinates")
	require.NotNil(t, weatherData, "Weather data should be returned")
}