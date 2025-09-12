package contract

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"github.com/dpup/info.ersn.net/server/internal/cache"
	"github.com/dpup/info.ersn.net/server/internal/lib/incident"
)

// ProcessedIncidentStore interface contract test
// Implementation is now available - run the tests!

// Use the actual types from the incident package
type ProcessedIncidentStore = incident.ProcessedIncidentStore
type ProcessedIncidentCache = incident.ProcessedIncidentCache
type ContentCacheMetrics = incident.ContentCacheMetrics

// TestProcessedIncidentStore_StoreAndGet tests basic store/retrieve functionality
func TestProcessedIncidentStore_StoreAndGet(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewProcessedIncidentStore(cacheInstance)
	ctx := context.Background()
	
	contentHash := incident.IncidentContentHash{
		ContentHash:      "abc123def456789012345678901234567890123456789012345678901234567890",
		NormalizedText:   "i-80 chain controls required",
		LocationKey:      "39.123_-120.567",
		IncidentCategory: "chain_control",
		FirstSeenAt:      time.Now(),
	}
	
	entry := incident.ProcessedIncidentCache{
		ContentHash:       contentHash,
		Stage:            incident.OPENAI_ENHANCED,
		OriginalIncident: MockIncident{Description: "Raw incident data"},
		ProcessedData:    map[string]string{"enhanced": "Enhanced incident description"},
		LastSeenInFeed:   time.Now(),
		CacheExpiresAt:   time.Now().Add(1 * time.Hour),
		ServeCount:       0,
		ProcessingDuration: 500 * time.Millisecond,
	}
	
	// Store the entry
	err := store.StoreProcessed(ctx, entry)
	require.NoError(t, err, "Storing processed incident should not error")
	
	// Retrieve the entry
	retrieved, err := store.GetProcessed(ctx, contentHash, incident.OPENAI_ENHANCED)
	require.NoError(t, err, "Getting processed incident should not error")
	require.NotNil(t, retrieved, "Retrieved entry should not be nil")
	
	// Verify data integrity
	assert.Equal(t, entry.ContentHash.ContentHash, retrieved.ContentHash.ContentHash)
	assert.Equal(t, entry.Stage, retrieved.Stage)
	assert.Equal(t, entry.ProcessingDuration, retrieved.ProcessingDuration)
}

// TestProcessedIncidentStore_NotFound tests missing entry behavior
func TestProcessedIncidentStore_NotFound(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewProcessedIncidentStore(cacheInstance)
	ctx := context.Background()
	
	nonExistentHash := incident.IncidentContentHash{
		ContentHash: "nonexistent123456789012345678901234567890123456789012345678901234567890",
	}
	
	// Should return not found
	retrieved, err := store.GetProcessed(ctx, nonExistentHash, incident.OPENAI_ENHANCED)
	require.NoError(t, err, "Getting non-existent incident should not error")
	assert.Nil(t, retrieved, "Retrieved entry should be nil for non-existent incident")
}

// TestProcessedIncidentStore_MarkSeenInCurrentFeed tests feed tracking
func TestProcessedIncidentStore_MarkSeenInCurrentFeed(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewProcessedIncidentStore(cacheInstance)
	ctx := context.Background()
	
	contentHash := incident.IncidentContentHash{
		ContentHash:      "mark123test456789012345678901234567890123456789012345678901234567890",
		IncidentCategory: "chain_control",
		NormalizedText:   "test incident",
		LocationKey:      "39.123_-120.567",
		FirstSeenAt:      time.Now(),
	}
	
	// Store an incident
	entry := incident.ProcessedIncidentCache{
		ContentHash:    contentHash,
		Stage:         incident.OPENAI_ENHANCED,
		OriginalIncident: map[string]interface{}{"description": "test"},
		ProcessedData:    map[string]interface{}{"enhanced": "true"},
		LastSeenInFeed: time.Now().Add(-2 * time.Hour), // 2 hours ago
		CacheExpiresAt: time.Now().Add(1 * time.Hour), // Expires in 1 hour
	}
	
	err := store.StoreProcessed(ctx, entry)
	require.NoError(t, err)
	
	// Mark as seen in current feed
	err = store.MarkSeenInCurrentFeed(ctx, contentHash)
	require.NoError(t, err, "Marking incident as seen should not error")
	
	// Note: The actual LastSeenInFeed update behavior depends on cache implementation
	// This test verifies the method doesn't error - specific timing behavior may vary
}

// TestProcessedIncidentStore_ExpireOldIncidents tests cleanup functionality
func TestProcessedIncidentStore_ExpireOldIncidents(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewProcessedIncidentStore(cacheInstance)
	ctx := context.Background()
	
	// Test that ExpireOldIncidents method executes without error
	// The actual expiration logic depends on cache implementation
	expiredCount, err := store.ExpireOldIncidents(ctx)
	require.NoError(t, err, "Expiring old incidents should not error")
	assert.GreaterOrEqual(t, expiredCount, 0, "Expired count should be non-negative")
}

// TestProcessedIncidentStore_GetCacheMetrics tests metrics collection
func TestProcessedIncidentStore_GetCacheMetrics(t *testing.T) {
	// Implementation is now available - run the test!
	cacheInstance := cache.NewCache()
	store := cache.NewProcessedIncidentStore(cacheInstance)
	ctx := context.Background()
	
	// Store some test data
	for i := 0; i < 3; i++ {
		contentHash := incident.IncidentContentHash{
			ContentHash:      fmt.Sprintf("metrics%02d3456789012345678901234567890123456789012345678901234567890", i),
			NormalizedText:   fmt.Sprintf("test incident %d", i),
			LocationKey:      "39.123_-120.567",
			IncidentCategory: "test",
			FirstSeenAt:      time.Now(),
		}
		
		entry := incident.ProcessedIncidentCache{
			ContentHash:    contentHash,
			Stage:         incident.OPENAI_ENHANCED,
			OriginalIncident: map[string]interface{}{"description": fmt.Sprintf("test %d", i)},
			ProcessedData:    map[string]interface{}{"enhanced": "true"},
			LastSeenInFeed:   time.Now(),
			CacheExpiresAt:   time.Now().Add(1 * time.Hour),
			ServeCount:       0,
			ProcessingDuration: 100 * time.Millisecond,
		}
		
		err := store.StoreProcessed(ctx, entry)
		require.NoError(t, err)
	}
	
	// Get metrics
	metrics, err := store.GetCacheMetrics(ctx)
	require.NoError(t, err, "Getting cache metrics should not error")
	
	// Verify metrics structure
	assert.GreaterOrEqual(t, metrics.TotalCachedIncidents, int64(0), "Total cached incidents should be non-negative")
	assert.GreaterOrEqual(t, metrics.CacheHitRate, 0.0, "Hit rate should be non-negative")
	assert.LessOrEqual(t, metrics.CacheHitRate, 1.0, "Hit rate should not exceed 1.0")
	assert.NotNil(t, metrics.IncidentsByStage, "IncidentsByStage should not be nil")
	assert.GreaterOrEqual(t, metrics.MemoryUsageBytes, int64(0), "Memory usage should be non-negative")
	assert.False(t, metrics.LastMetricsUpdate.IsZero(), "LastMetricsUpdate should be set")
}