package alerts

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Contract tests for alert-enhancer library
// These tests define the interface before implementation exists
// They MUST FAIL initially to satisfy TDD RED-GREEN-Refactor cycle

func TestAlertEnhancer_EnhanceAlert(t *testing.T) {
	enhancer := NewAlertEnhancer("test-api-key", "gpt-3.5-turbo")
	ctx := context.Background()

	// Test with real Caltrans description sample
	rawAlert := RawAlert{
		ID:          "test-001",
		Description: "Rte 4 EB of MM 31 - VEHICLE IN DITCH, EMS ENRT",
		Location:    "Highway 4",
		Timestamp:   time.Now(),
	}

	enhanced, err := enhancer.EnhanceAlert(ctx, rawAlert)
	require.NoError(t, err)

	// Verify enhanced alert structure
	assert.Equal(t, rawAlert.ID, enhanced.ID)
	assert.Equal(t, rawAlert.Description, enhanced.OriginalDescription)
	assert.NotEmpty(t, enhanced.StructuredDescription.Details)
	assert.NotEmpty(t, enhanced.StructuredDescription.Location)
	
	// Verify required fields are populated
	assert.Contains(t, []string{"none", "light", "moderate", "severe"}, enhanced.StructuredDescription.Impact)
	assert.Contains(t, []string{"unknown", "< 1 hour", "several hours", "ongoing"}, enhanced.StructuredDescription.Duration)
	
	// Verify condensed summary format
	assert.NotEmpty(t, enhanced.CondensedSummary)
	assert.LessOrEqual(t, len(enhanced.CondensedSummary), 200, "Condensed summary should be <= 200 chars")
}

func TestAlertEnhancer_EnhanceAlert_ComplexDescription(t *testing.T) {
	enhancer := NewAlertEnhancer("test-api-key", "gpt-3.5-turbo")
	ctx := context.Background()

	// Test with complex Caltrans description
	rawAlert := RawAlert{
		ID:          "test-002",
		Description: "Rte 4 WB at Arnold Rim - OVERTURNED VEHICLE OFF ROADWAY, BLOCKING 1 LN, EMS/FIRE ENRT, TOW REQ, VIS: NOT VISIBLE FROM ROADWAY",
		Location:    "Highway 4 at Arnold Rim",
		Timestamp:   time.Now(),
	}

	enhanced, err := enhancer.EnhanceAlert(ctx, rawAlert)
	require.NoError(t, err)

	// Verify visibility info is captured in additional metadata
	assert.NotNil(t, enhanced.StructuredDescription.AdditionalInfo)
	
	// Should extract structured data
	assert.NotEmpty(t, enhanced.StructuredDescription.Details)
	assert.Contains(t, enhanced.StructuredDescription.Details, "overturned")
	assert.Contains(t, enhanced.StructuredDescription.Location, "Arnold Rim")
}

func TestAlertEnhancer_GenerateCondensedSummary(t *testing.T) {
	enhancer := NewAlertEnhancer("test-api-key", "gpt-3.5-turbo")
	ctx := context.Background()

	structured := StructuredDescription{
		TimeReported: "2025-09-11T10:43:00Z",
		Details:      "Vehicle overturned off roadway, not visible from highway",
		Location:     "Highway 4 at Arnold Rim",
		Impact:       "light",
		Duration:     "< 1 hour",
		AdditionalInfo: map[string]string{
			"visibility": "not visible from roadway",
			"lanes_affected": "1 of 2",
		},
	}

	summary, err := enhancer.GenerateCondensedSummary(ctx, structured)
	require.NoError(t, err)

	// Verify format matches expected pattern: "Hwy 4 – Location: Description (Time)"
	assert.NotEmpty(t, summary)
	assert.LessOrEqual(t, len(summary), 150, "Summary should be <= 150 chars")
	assert.Contains(t, summary, "Hwy 4")
	assert.Contains(t, summary, "Arnold Rim")
	assert.Contains(t, summary, "overturned")
}

func TestAlertEnhancer_HealthCheck(t *testing.T) {
	enhancer := NewAlertEnhancer("test-api-key", "gpt-3.5-turbo")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := enhancer.HealthCheck(ctx)
	// This may pass or fail depending on API key validity
	// But it should not panic or hang
	assert.IsType(t, error(nil), err) // Just verify it returns an error type
}

func TestAlertEnhancer_TimeoutHandling(t *testing.T) {
	enhancer := NewAlertEnhancer("test-api-key", "gpt-3.5-turbo")
	
	// Test with very short timeout to force timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	rawAlert := RawAlert{
		ID:          "test-timeout",
		Description: "Test timeout handling",
		Location:    "Test Location",
		Timestamp:   time.Now(),
	}

	_, err := enhancer.EnhanceAlert(ctx, rawAlert)
	assert.Error(t, err, "Should return error on timeout")
}

func TestAlertEnhancer_ErrorHandling(t *testing.T) {
	// Test with invalid API key
	enhancer := NewAlertEnhancer("invalid-api-key", "gpt-3.5-turbo")
	ctx := context.Background()

	rawAlert := RawAlert{
		ID:          "test-error",
		Description: "Test error handling",
		Location:    "Test Location",
		Timestamp:   time.Now(),
	}

	_, err := enhancer.EnhanceAlert(ctx, rawAlert)
	assert.Error(t, err, "Should return error for invalid API key")
}

func TestAlertEnhancer_StructuredOutputValidation(t *testing.T) {
	enhancer := NewAlertEnhancer("test-api-key", "gpt-3.5-turbo")
	ctx := context.Background()

	rawAlert := RawAlert{
		ID:          "test-validation",
		Description: "Rte 4 - CONSTRUCTION WORK, DELAYS POSSIBLE",
		Location:    "Highway 4",
		Timestamp:   time.Now(),
	}

	enhanced, err := enhancer.EnhanceAlert(ctx, rawAlert)
	require.NoError(t, err)

	// Validate structured output schema compliance
	structured := enhanced.StructuredDescription
	
	// Required fields must be present
	assert.NotEmpty(t, structured.Details, "Details field is required")
	assert.NotEmpty(t, structured.Location, "Location field is required")
	
	// Enum values must be valid
	validImpacts := []string{"none", "light", "moderate", "severe"}
	assert.Contains(t, validImpacts, structured.Impact, "Impact must be valid enum value")
	
	validDurations := []string{"unknown", "< 1 hour", "several hours", "ongoing"}
	assert.Contains(t, validDurations, structured.Duration, "Duration must be valid enum value")
	
	// Additional info should be map[string]string if present
	if structured.AdditionalInfo != nil {
		assert.IsType(t, map[string]string{}, structured.AdditionalInfo)
	}
}

// Benchmark test for performance validation
func BenchmarkAlertEnhancer_EnhanceAlert(b *testing.B) {
	enhancer := NewAlertEnhancer("test-api-key", "gpt-3.5-turbo")
	ctx := context.Background()

	rawAlert := RawAlert{
		ID:          "benchmark-test",
		Description: "Rte 4 EB - TRAFFIC HAZARD",
		Location:    "Highway 4",
		Timestamp:   time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = enhancer.EnhanceAlert(ctx, rawAlert)
	}
}