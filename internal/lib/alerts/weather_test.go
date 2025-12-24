package alerts

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OpenWeatherOneCallResponse represents the One Call API response structure
type OpenWeatherOneCallResponse struct {
	Alerts []OpenWeatherAlertData `json:"alerts"`
}

// OpenWeatherAlertData represents a single alert from the API
type OpenWeatherAlertData struct {
	SenderName  string   `json:"sender_name"`
	Event       string   `json:"event"`
	Start       int64    `json:"start"`
	End         int64    `json:"end"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

func TestWeatherAlertEnhancer_ParsesFixtureData(t *testing.T) {
	// Load fixture data
	fixtureFiles := []string{
		"../../../tests/testdata/weather/bearvalley_alerts_20251224.json",
		"../../../tests/testdata/weather/arnold_alerts_20251224.json",
		"../../../tests/testdata/weather/murphys_alerts_20251224.json",
	}

	for _, fixturePath := range fixtureFiles {
		t.Run(fixturePath, func(t *testing.T) {
			data, err := os.ReadFile(fixturePath)
			if os.IsNotExist(err) {
				t.Skip("Fixture file not found:", fixturePath)
			}
			require.NoError(t, err)

			var response OpenWeatherOneCallResponse
			err = json.Unmarshal(data, &response)
			require.NoError(t, err)

			// Verify we have alerts in the fixture
			assert.NotEmpty(t, response.Alerts, "Expected alerts in fixture data")

			// Verify each alert has required fields
			for i, alert := range response.Alerts {
				assert.NotEmpty(t, alert.Event, "Alert %d should have event", i)
				assert.NotEmpty(t, alert.Description, "Alert %d should have description", i)
				assert.NotEmpty(t, alert.SenderName, "Alert %d should have sender_name", i)
			}
		})
	}
}

func TestWeatherAlertEnhancer_NoAPIKey(t *testing.T) {
	// Test that enhancer returns error with no API key
	enhancer := NewWeatherAlertEnhancer("", "gpt-4o-mini")

	rawAlert := RawWeatherAlert{
		ID:          "test-1",
		Event:       "Winter Storm Warning",
		SenderName:  "NWS Sacramento CA",
		Description: "Heavy snow expected.",
		Tags:        []string{"Snow/Ice"},
	}

	_, err := enhancer.EnhanceWeatherAlert(context.Background(), rawAlert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRawWeatherAlert_Structure(t *testing.T) {
	// Test RawWeatherAlert structure matches what we need
	alert := RawWeatherAlert{
		ID:          "test-123",
		Event:       "Flood Watch",
		SenderName:  "NWS Sacramento CA",
		Description: "* WHAT...Flooding possible.",
		Tags:        []string{"Flood"},
		Start:       1766518440,
		End:         1766793600,
	}

	assert.Equal(t, "test-123", alert.ID)
	assert.Equal(t, "Flood Watch", alert.Event)
	assert.Equal(t, "NWS Sacramento CA", alert.SenderName)
	assert.Contains(t, alert.Description, "Flooding")
	assert.Contains(t, alert.Tags, "Flood")
	assert.Equal(t, int64(1766518440), alert.Start)
	assert.Equal(t, int64(1766793600), alert.End)
}

func TestEnhancedWeatherAlert_Structure(t *testing.T) {
	// Test EnhancedWeatherAlert structure
	enhanced := EnhancedWeatherAlert{
		ID:       "test-123",
		Headline: "Heavy snow and 60 mph winds through Friday",
		Summary:  "Expect dangerous travel conditions with 4-8 feet of snow.",
		Details:  "**Snow amounts:** Up to 1 foot...",
	}

	assert.Equal(t, "test-123", enhanced.ID)
	assert.NotEmpty(t, enhanced.Headline)
	assert.NotEmpty(t, enhanced.Summary)
	assert.NotEmpty(t, enhanced.Details)
	assert.Less(t, len(enhanced.Headline), 100, "Headline should be under 100 chars")
}

func TestWeatherAlertSystemPrompt_NotEmpty(t *testing.T) {
	// Verify the system prompt is defined and contains key instructions
	assert.NotEmpty(t, WeatherAlertSystemPrompt)
	assert.Contains(t, WeatherAlertSystemPrompt, "headline")
	assert.Contains(t, WeatherAlertSystemPrompt, "summary")
	assert.Contains(t, WeatherAlertSystemPrompt, "details")
	assert.Contains(t, WeatherAlertSystemPrompt, "Winter")
	assert.Contains(t, WeatherAlertSystemPrompt, "Flood")
}

func TestWeatherAlertEnhancementSchema_Valid(t *testing.T) {
	// Verify the schema name is set correctly
	assert.Equal(t, "weather_alert_enhancement", WeatherAlertEnhancementSchema.Name)
	assert.True(t, WeatherAlertEnhancementSchema.Strict)
}
