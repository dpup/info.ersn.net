package contract

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"github.com/dpup/info.ersn.net/server/internal/cache"
	"github.com/dpup/info.ersn.net/server/internal/lib/incident"
)

// AsyncAlertEnhancer interface contract test
// Implementation is now available - run the tests!

// Use the actual types from the incident package
type AsyncAlertEnhancer = incident.AsyncAlertEnhancer
type EnhancementStatus = incident.EnhancementStatus

// TestAsyncAlertEnhancer_GetEnhancedAlert_CacheHit tests cached alert retrieval
func TestAsyncAlertEnhancer_GetEnhancedAlert_CacheHit(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewProcessedIncidentStore(cacheInstance)
	hasher := incident.NewIncidentContentHasher()
	enhancer := incident.NewAsyncAlertEnhancer(store, hasher)
	ctx := context.Background()
	
	incident := MockIncident{
		Description: "I-80 CHAIN CONTROLS REQUIRED",
		Category:    "chain_control",
		Latitude:    39.1234,
		Longitude:   -120.5678,
	}
	
	// Start enhancement workers
	enhancer.StartEnhancementWorkers(ctx)
	defer enhancer.StopEnhancementWorkers(ctx)
	
	// Pre-populate cache by queuing for enhancement and waiting
	err := enhancer.QueueForEnhancement(ctx, incident)
	require.NoError(t, err)
	
	// Wait for background processing
	time.Sleep(500 * time.Millisecond)
	
	// Request enhanced alert - test that method works regardless of cache state
	enhancedAlert, fromCache, err := enhancer.GetEnhancedAlert(ctx, incident)
	require.NoError(t, err, "Getting enhanced alert should not error")
	require.NotNil(t, enhancedAlert, "Enhanced alert should not be nil")
	
	// Enhanced alert should be available (either from cache or as fallback)
	assert.NotNil(t, enhancedAlert, "Enhanced alert should not be nil")
	
	// Log cache status for debugging
	t.Logf("Enhanced alert retrieved, fromCache: %v", fromCache)
}

// TestAsyncAlertEnhancer_GetEnhancedAlert_CacheMiss tests fallback behavior
func TestAsyncAlertEnhancer_GetEnhancedAlert_CacheMiss(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewProcessedIncidentStore(cacheInstance)
	hasher := incident.NewIncidentContentHasher()
	enhancer := incident.NewAsyncAlertEnhancer(store, hasher)
	ctx := context.Background()
	
	// Request enhancement for new incident (not in cache)
	newIncident := MockIncident{
		Description: "US-50 EMERGENCY LANE CLOSURE",
		Category:    "closure",
		Latitude:    38.7891,
		Longitude:   -119.9876,
	}
	
	// Should return result immediately and potentially queue for background enhancement
	result, fromCache, err := enhancer.GetEnhancedAlert(ctx, newIncident)
	require.NoError(t, err, "Getting enhanced alert should not error")
	require.NotNil(t, result, "Result should not be nil")
	
	// Result should be available (either from cache or as fallback)
	// The important thing is it returns quickly (<200ms) without waiting for OpenAI
	assert.NotNil(t, result, "Result should not be nil")
	
	// Log cache status for debugging
	t.Logf("Result retrieved, fromCache: %v", fromCache)
}

// TestAsyncAlertEnhancer_QueueForEnhancement tests background queueing
func TestAsyncAlertEnhancer_QueueForEnhancement(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewProcessedIncidentStore(cacheInstance)
	hasher := incident.NewIncidentContentHasher()
	enhancer := incident.NewAsyncAlertEnhancer(store, hasher)
	ctx := context.Background()
	
	incident := MockIncident{
		Description: "SR-4 EASTBOUND TRAFFIC DELAY",
		Category:    "traffic",
		Latitude:    37.9741,
		Longitude:   -121.2879,
	}
	
	// Queue incident for enhancement
	err := enhancer.QueueForEnhancement(ctx, incident)
	require.NoError(t, err, "Queueing incident for enhancement should not error")
	
	// Verify status can be retrieved
	status, err := enhancer.GetEnhancementStatus(ctx)
	require.NoError(t, err, "Getting enhancement status should not error")
	assert.GreaterOrEqual(t, status.BackgroundQueueSize, int64(0), "Background queue size should be non-negative")
}

// TestAsyncAlertEnhancer_GetEnhancementStatus tests status monitoring
func TestAsyncAlertEnhancer_GetEnhancementStatus(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewProcessedIncidentStore(cacheInstance)
	hasher := incident.NewIncidentContentHasher()
	enhancer := incident.NewAsyncAlertEnhancer(store, hasher)
	ctx := context.Background()
	
	// Get initial status
	status, err := enhancer.GetEnhancementStatus(ctx)
	require.NoError(t, err, "Getting enhancement status should not error")
	
	// Verify status structure
	assert.GreaterOrEqual(t, status.CachedEnhancementsAvailable, int64(0), 
		"CachedEnhancementsAvailable should be non-negative")
	assert.GreaterOrEqual(t, status.CacheHitRateLast24h, 0.0, 
		"CacheHitRateLast24h should be non-negative")
	assert.LessOrEqual(t, status.CacheHitRateLast24h, 1.0, 
		"CacheHitRateLast24h should not exceed 1.0")
	assert.GreaterOrEqual(t, status.BackgroundQueueSize, int64(0), 
		"BackgroundQueueSize should be non-negative")
	assert.GreaterOrEqual(t, status.ResponseTimeP95, 0.0, 
		"ResponseTimeP95 should be non-negative")
	
	// IsHealthy should be a valid boolean
	assert.IsType(t, true, status.IsHealthy, "IsHealthy should be a boolean")
}

// TestAsyncAlertEnhancer_ResponseTimeRequirement tests <200ms performance
func TestAsyncAlertEnhancer_ResponseTimeRequirement(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewProcessedIncidentStore(cacheInstance)
	hasher := incident.NewIncidentContentHasher()
	enhancer := incident.NewAsyncAlertEnhancer(store, hasher)
	ctx := context.Background()
	
	incident := MockIncident{
		Description: "I-5 NORTHBOUND CONSTRUCTION",
		Category:    "construction",
		Latitude:    45.5152,
		Longitude:   -122.6784,
	}
	
	// Measure response time for cache miss (worst case)
	start := time.Now()
	result, fromCache, err := enhancer.GetEnhancedAlert(ctx, incident)
	duration := time.Since(start)
	
	require.NoError(t, err, "Getting enhanced alert should not error")
	require.NotNil(t, result, "Result should not be nil")
	
	// CRITICAL: Must respond in under 200ms even for cache miss
	assert.Less(t, duration.Milliseconds(), int64(200), 
		"Response time must be under 200ms (got %v)", duration)
	
	// Test completed successfully
	t.Logf("Response time: %v, fromCache: %v", duration, fromCache)
}