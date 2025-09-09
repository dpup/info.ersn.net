package contract

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	api "github.com/dpup/info.ersn.net/server"
	"github.com/dpup/info.ersn.net/server/internal/services"
)

// T006: Contract test WeatherService.ListWeather - MUST FAIL initially  
func TestWeatherService_ListWeather_Contract(t *testing.T) {
	// This test MUST fail until WeatherService is implemented
	service := &services.WeatherService{} // No implementation yet - will cause compilation error

	req := &api.ListWeatherRequest{}
	resp, err := service.ListWeather(context.Background(), req)

	// Contract requirements from contracts/weather.proto lines 12-17
	require.NoError(t, err, "ListWeather should not return error")
	require.NotNil(t, resp, "Response should not be nil")
	require.NotNil(t, resp.WeatherData, "WeatherData array should not be nil")
	require.NotNil(t, resp.LastUpdated, "LastUpdated timestamp should not be nil")
	
	// Validate response structure matches proto definition
	if len(resp.WeatherData) > 0 {
		weather := resp.WeatherData[0]
		require.NotEmpty(t, weather.LocationId, "Location ID should not be empty")
		require.NotEmpty(t, weather.LocationName, "Location name should not be empty")
		require.NotNil(t, weather.Coordinates, "Coordinates should not be nil")
		require.NotEmpty(t, weather.WeatherMain, "Weather main should not be empty")
		require.NotEmpty(t, weather.WeatherDescription, "Weather description should not be empty")
		
		// Temperature should be reasonable (consistent units: Celsius)
		require.Greater(t, weather.TemperatureCelsius, float32(-50), "Temperature should be above absolute minimum")
		require.Less(t, weather.TemperatureCelsius, float32(60), "Temperature should be below extreme maximum")
		
		// Coordinates should be valid WGS84 decimal degrees
		require.GreaterOrEqual(t, weather.Coordinates.Latitude, -90.0, "Latitude should be >= -90")
		require.LessOrEqual(t, weather.Coordinates.Latitude, 90.0, "Latitude should be <= 90")
		require.GreaterOrEqual(t, weather.Coordinates.Longitude, -180.0, "Longitude should be >= -180")
		require.LessOrEqual(t, weather.Coordinates.Longitude, 180.0, "Longitude should be <= 180")
	}
}

// T007: Contract test WeatherService.ListWeatherAlerts - MUST FAIL initially
func TestWeatherService_ListWeatherAlerts_Contract(t *testing.T) {
	// This test MUST fail until WeatherService is implemented
	service := &services.WeatherService{} // No implementation yet - will cause compilation error

	req := &api.ListWeatherAlertsRequest{}
	resp, err := service.ListWeatherAlerts(context.Background(), req)

	// Contract requirements from contracts/weather.proto lines 26-31
	require.NoError(t, err, "ListWeatherAlerts should not return error")
	require.NotNil(t, resp, "Response should not be nil")
	require.NotNil(t, resp.Alerts, "Alerts array should not be nil")
	require.NotNil(t, resp.LastUpdated, "LastUpdated timestamp should not be nil")
	
	// Validate alert structure if alerts present
	if len(resp.Alerts) > 0 {
		alert := resp.Alerts[0]
		require.NotEmpty(t, alert.Id, "Alert ID should not be empty")
		require.NotEmpty(t, alert.SenderName, "Sender name should not be empty")
		require.NotEmpty(t, alert.Event, "Event should not be empty")
		require.Greater(t, alert.StartTimestamp, int64(0), "Start timestamp should be positive")
		require.Greater(t, alert.EndTimestamp, alert.StartTimestamp, "End timestamp should be after start")
		
		// Tags array should exist (may be empty)
		require.NotNil(t, alert.Tags, "Tags array should not be nil")
	}
}