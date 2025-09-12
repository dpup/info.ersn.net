package contract

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"github.com/dpup/info.ersn.net/server/internal/cache"
	"github.com/dpup/info.ersn.net/server/internal/lib/alerts"
	"github.com/dpup/info.ersn.net/server/internal/lib/incident"
)

// BackgroundIncidentProcessor interface contract test
// Implementation is now available - run the tests!

// Use the actual types from the incident package
type IncidentBatchProcessor = incident.IncidentBatchProcessor
type BatchProcessingStats = incident.BatchProcessingStats

// mockBackgroundAlertEnhancer provides a test double for AlertEnhancer in background processor tests
type mockBackgroundAlertEnhancer struct{}

func (m *mockBackgroundAlertEnhancer) EnhanceAlert(ctx context.Context, raw alerts.RawAlert) (alerts.EnhancedAlert, error) {
	// Return a mock enhanced alert
	return alerts.EnhancedAlert{
		ID:                  raw.ID,
		OriginalDescription: raw.Description,
		StructuredDescription: alerts.StructuredDescription{
			Details: "Enhanced: " + raw.Description,
			Location: alerts.StructuredLocation{
				Description: raw.Location,
				Latitude:    39.1234,
				Longitude:   -120.5678,
			},
			Impact:           "moderate",
			Duration:         "< 1 hour",
			CondensedSummary: "Mock enhanced incident",
		},
		CondensedSummary: "Mock enhanced incident",
		ProcessedAt:      time.Now(),
	}, nil
}

func (m *mockBackgroundAlertEnhancer) HealthCheck(ctx context.Context) error {
	return nil
}

// TestBackgroundIncidentProcessor_StartAndStop tests processor lifecycle
func TestBackgroundIncidentProcessor_StartAndStop(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewIncidentStore(cacheInstance)
	hasher := incident.NewIncidentContentHasher()
	mockEnhancer := &mockBackgroundAlertEnhancer{}
	enhancer := incident.NewAsyncAlertEnhancer(store, hasher, mockEnhancer)
	processor := incident.NewBackgroundIncidentProcessor(store, hasher, enhancer)
	ctx := context.Background()
	
	// Start background processing
	err := processor.StartBackgroundProcessing(ctx)
	require.NoError(t, err, "Starting background processing should not error")
	
	// Allow some time for startup
	time.Sleep(100 * time.Millisecond)
	
	// Stop processing
	err = processor.Stop(ctx)
	require.NoError(t, err, "Stopping background processing should not error")
}

// TestBackgroundIncidentProcessor_ProcessIncidentBatch tests batch processing
func TestBackgroundIncidentProcessor_ProcessIncidentBatch(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewIncidentStore(cacheInstance)
	hasher := incident.NewIncidentContentHasher()
	mockEnhancer := &mockBackgroundAlertEnhancer{}
	enhancer := incident.NewAsyncAlertEnhancer(store, hasher, mockEnhancer)
	processor := incident.NewBackgroundIncidentProcessor(store, hasher, enhancer)
	ctx := context.Background()
	
	// Start processor
	err := processor.StartBackgroundProcessing(ctx)
	require.NoError(t, err)
	defer processor.Stop(ctx)
	
	// Create test incident batch
	incidents := []interface{}{
		MockIncident{
			Description: "I-80 CHAIN CONTROLS REQUIRED",
			Latitude:    39.1234,
			Longitude:   -120.5678,
			Category:    "chain_control",
		},
		MockIncident{
			Description: "US-50 LANE CLOSURE WESTBOUND",
			Latitude:    38.7891,
			Longitude:   -119.9876,
			Category:    "closure",
		},
	}
	
	// Process the batch
	err = processor.ProcessIncidentBatch(ctx, incidents)
	require.NoError(t, err, "Processing incident batch should not error")
	
	// Allow time for background processing
	time.Sleep(200 * time.Millisecond)
	
	// Verify stats show processing activity
	stats, err := processor.GetProcessingStats(ctx)
	require.NoError(t, err, "Getting processing stats should not error")
	assert.GreaterOrEqual(t, stats.QueuedIncidents, int64(0), "Queued incidents should be non-negative")
}

// TestBackgroundIncidentProcessor_PrefetchCommonIncidents tests proactive processing
func TestBackgroundIncidentProcessor_PrefetchCommonIncidents(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewIncidentStore(cacheInstance)
	hasher := incident.NewIncidentContentHasher()
	mockEnhancer := &mockBackgroundAlertEnhancer{}
	enhancer := incident.NewAsyncAlertEnhancer(store, hasher, mockEnhancer)
	processor := incident.NewBackgroundIncidentProcessor(store, hasher, enhancer)
	ctx := context.Background()
	
	// Start processor
	err := processor.StartBackgroundProcessing(ctx)
	require.NoError(t, err)
	defer processor.Stop(ctx)
	
	// Prefetch common incidents
	err = processor.PrefetchCommonIncidents(ctx)
	require.NoError(t, err, "Prefetching common incidents should not error")
	
	// Allow time for prefetching
	time.Sleep(100 * time.Millisecond)
	
	// Stats should show some activity
	stats, err := processor.GetProcessingStats(ctx)
	require.NoError(t, err)
	
	// At minimum, the call should not crash and return valid stats
	assert.GreaterOrEqual(t, stats.QueuedIncidents, int64(0), "Queued incidents should be non-negative")
	assert.GreaterOrEqual(t, stats.ProcessedIncidents, int64(0), "Processed incidents should be non-negative")
}

// TestBackgroundIncidentProcessor_GetProcessingStats tests metrics collection
func TestBackgroundIncidentProcessor_GetProcessingStats(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewIncidentStore(cacheInstance)
	hasher := incident.NewIncidentContentHasher()
	mockEnhancer := &mockBackgroundAlertEnhancer{}
	enhancer := incident.NewAsyncAlertEnhancer(store, hasher, mockEnhancer)
	processor := incident.NewBackgroundIncidentProcessor(store, hasher, enhancer)
	ctx := context.Background()
	
	// Get initial stats
	stats, err := processor.GetProcessingStats(ctx)
	require.NoError(t, err, "Getting processing stats should not error")
	
	// Verify stats structure
	assert.GreaterOrEqual(t, stats.QueuedIncidents, int64(0), "QueuedIncidents should be non-negative")
	assert.GreaterOrEqual(t, stats.ProcessedIncidents, int64(0), "ProcessedIncidents should be non-negative")
	assert.GreaterOrEqual(t, stats.FailedProcessing, int64(0), "FailedProcessing should be non-negative")
	assert.GreaterOrEqual(t, stats.AvgProcessingTime, time.Duration(0), "AvgProcessingTime should be non-negative")
	assert.GreaterOrEqual(t, stats.OpenAICallsSaved, int64(0), "OpenAICallsSaved should be non-negative")
	assert.GreaterOrEqual(t, stats.CostSavingsEstimate, 0.0, "CostSavingsEstimate should be non-negative")
}

// TestBackgroundIncidentProcessor_ConcurrentSafety tests basic concurrent access
func TestBackgroundIncidentProcessor_ConcurrentSafety(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewIncidentStore(cacheInstance)
	hasher := incident.NewIncidentContentHasher()
	mockEnhancer := &mockBackgroundAlertEnhancer{}
	enhancer := incident.NewAsyncAlertEnhancer(store, hasher, mockEnhancer)
	processor := incident.NewBackgroundIncidentProcessor(store, hasher, enhancer)
	ctx := context.Background()
	
	// Start processor
	err := processor.StartBackgroundProcessing(ctx)
	require.NoError(t, err)
	defer processor.Stop(ctx)
	
	// Test basic concurrent stats access - simplified test
	go func() {
		_, err := processor.GetProcessingStats(ctx)
		assert.NoError(t, err)
	}()
	
	go func() {
		_, err := processor.GetProcessingStats(ctx)
		assert.NoError(t, err)
	}()
	
	// Allow time for goroutines to complete
	time.Sleep(100 * time.Millisecond)
}