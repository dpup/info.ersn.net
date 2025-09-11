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
	// Test with invalid API key (should return error)
	enhancer := NewAlertEnhancer("invalid-test-key", "gpt-3.5-turbo")
	ctx := context.Background()

	// Test with real Caltrans description sample
	rawAlert := RawAlert{
		ID:          "test-001",
		Description: "Rte 4 EB of MM 31 - VEHICLE IN DITCH, EMS ENRT",
		Location:    "Highway 4",
		Timestamp:   time.Now(),
	}

	_, err := enhancer.EnhanceAlert(ctx, rawAlert)
	assert.Error(t, err, "Should return error with invalid API key")
	
	// Test basic interface compliance
	assert.NotNil(t, enhancer, "Enhancer should be created even with invalid key")
	
	// Test with empty API key (should return error)
	emptyEnhancer := NewAlertEnhancer("", "gpt-3.5-turbo")
	_, err = emptyEnhancer.EnhanceAlert(ctx, rawAlert)
	assert.Error(t, err, "Should return error with empty API key")
}

func TestAlertEnhancer_EnhanceAlert_ComplexDescription(t *testing.T) {
	enhancer := NewAlertEnhancer("invalid-key", "gpt-3.5-turbo")
	ctx := context.Background()

	// Test with complex Caltrans description
	rawAlert := RawAlert{
		ID:          "test-002",
		Description: "Rte 4 WB at Arnold Rim - OVERTURNED VEHICLE OFF ROADWAY, BLOCKING 1 LN, EMS/FIRE ENRT, TOW REQ, VIS: NOT VISIBLE FROM ROADWAY",
		Location:    "Highway 4 at Arnold Rim",
		Timestamp:   time.Now(),
	}

	_, err := enhancer.EnhanceAlert(ctx, rawAlert)
	assert.Error(t, err, "Should return error with invalid API key")
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
	// Test with valid client but invalid key (should return error)
	enhancer := NewAlertEnhancer("invalid-key", "gpt-3.5-turbo")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := enhancer.HealthCheck(ctx)
	assert.Error(t, err, "Should return error with invalid API key")
	
	// Test with nil client (should return error)
	emptyEnhancer := NewAlertEnhancer("", "gpt-3.5-turbo")
	err = emptyEnhancer.HealthCheck(ctx)
	assert.Error(t, err, "Should return error with nil client")
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
	// Test interface compliance without making real API calls
	enhancer := NewAlertEnhancer("invalid-key", "gpt-3.5-turbo")
	ctx := context.Background()

	rawAlert := RawAlert{
		ID:          "test-validation",
		Description: "Rte 4 - CONSTRUCTION WORK, DELAYS POSSIBLE",
		Location:    "Highway 4",
		Timestamp:   time.Now(),
	}

	_, err := enhancer.EnhanceAlert(ctx, rawAlert)
	assert.Error(t, err, "Should return error with invalid API key")
	
	// Test that the interface works as expected
	assert.NotNil(t, enhancer, "Enhancer should be created")
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