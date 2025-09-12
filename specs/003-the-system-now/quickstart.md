# Quickstart: Background Processing for Incident Content Caching

**Feature**: Background processing to achieve <200ms API response times  
**Date**: 2025-09-12  
**Status**: Ready for Implementation

## Overview

This quickstart guide demonstrates how the background incident processing system works to eliminate OpenAI processing delays from user-facing API requests. The system prefetches and processes Caltrans incident data in the background, serving cached results for <200ms response times.

## Prerequisites

- Go 1.21+
- OpenAI API key configured (`PF__ROADS__OPENAI__API_KEY`)
- Existing ERSN Info Server running
- Caltrans KML feeds accessible

## Architecture Flow

```
┌─ Caltrans KML Feeds ─┐    ┌── Background Processor ──┐    ┌─ API Response ─┐
│                      │    │                          │    │                │
│ • Chain Controls     │───▶│ 1. Content Hash Gen      │───▶│ Cached Results │
│ • Lane Closures      │    │ 2. Duplicate Detection   │    │ <200ms         │
│ • CHP Incidents      │    │ 3. OpenAI Enhancement    │    │                │
│                      │    │ 4. Cache Storage         │    │                │
└──────────────────────┘    └──────────────────────────┘    └────────────────┘
```

## Key Components Integration

### 1. **Content-Based Incident Identification**

The system creates deterministic content hashes to identify identical incidents across feed refreshes:

```go
// Example incident processing flow
incident := &CaltransIncident{
    Description: "I-80 WESTBOUND CHAIN CONTROLS IN EFFECT FROM DRUM FORESTHILL RD TO APPLEGATE/WEIMAR",
    Location:    Point{Lat: 39.1234, Lng: -120.5678},
    Category:    "chain_control",
}

// Generate content hash (deterministic)
contentHash := hasher.HashIncident(ctx, incident)
// Result: contentHash.ContentHash = "a1b2c3d4..." (SHA-256)
```

### 2. **Background Processing Pipeline**

When new incidents appear in Caltrans feeds:

```go
// Step 1: Feed refresh detects new incidents
newIncidents := caltransClient.ParseLatestFeeds(ctx)

// Step 2: Generate content hashes and check cache
for _, incident := range newIncidents {
    contentHash := hasher.HashIncident(ctx, incident)
    
    // Step 3: Check if already processed
    cached, found := cache.GetProcessed(ctx, contentHash, OPENAI_ENHANCED)
    if found {
        // Mark as seen to prevent expiration
        cache.MarkSeenInCurrentFeed(ctx, contentHash)
        continue
    }
    
    // Step 4: Queue for background OpenAI processing
    backgroundProcessor.QueueForEnhancement(ctx, incident)
}
```

### 3. **Fast API Response Flow**

User requests road conditions:

```go
// Step 1: Get current incidents (from cache or feeds)
incidents := roadsService.getCurrentIncidents(ctx, routeID)

// Step 2: Serve enhanced data from cache, raw data as fallback
enhancedIncidents := []EnhancedAlert{}
for _, incident := range incidents {
    contentHash := hasher.HashIncident(ctx, incident)
    
    // Try to get enhanced version from cache
    cached, found := cache.GetProcessed(ctx, contentHash, OPENAI_ENHANCED)
    if found && !cached.IsExpired() {
        enhancedIncidents = append(enhancedIncidents, cached.ProcessedData)
    } else {
        // Serve raw data immediately (<200ms requirement)
        enhancedIncidents = append(enhancedIncidents, incident.ToRawAlert())
        // Queue for background enhancement for next request
        backgroundProcessor.QueueForEnhancement(ctx, incident)
    }
}

return enhancedIncidents // Response time: <200ms
```

## Configuration Setup

Add to `prefab.yaml`:

```yaml
roads:
  store:
    processing_interval_minutes: 5      # Check for new incidents every 5 min
    max_concurrent_openai: 3           # Conservative rate limiting
    cache_ttl_hours: 1                 # Expire 1 hour after incident disappears  
    prefetch_enabled: true             # Proactively process common incidents
    openai_timeout_seconds: 30         # Timeout for individual OpenAI calls
```

Environment variables:
```bash
# Configure OpenAI rate limiting
export PF__ROADS__STORE__MAX_CONCURRENT_OPENAI=3

# Cache duration (1 hour after incident disappears from feeds)
export PF__ROADS__STORE__CACHE_TTL_HOURS=1

# Processing frequency (minutes)
export PF__ROADS__STORE__PROCESSING_INTERVAL_MINUTES=5
```

## Testing the Implementation

### Unit Tests (TDD - Must Fail First)

```go
// Test content hash generation
func TestIncidentContentHasher_HashIncident(t *testing.T) {
    // This test MUST fail until implementation exists
    hasher := NewIncidentContentHasher()
    
    incident := &CaltransIncident{
        Description: "I-80 Chain Controls",
        Location:    Point{Lat: 39.123, Lng: -120.567},
        Category:    "chain_control",
    }
    
    hash1, err := hasher.HashIncident(context.Background(), incident)
    require.NoError(t, err)
    
    // Same incident should produce same hash (deterministic)
    hash2, err := hasher.HashIncident(context.Background(), incident)
    require.NoError(t, err)
    require.Equal(t, hash1.ContentHash, hash2.ContentHash)
}

// Test background processing queue
func TestBackgroundIncidentProcessor_ProcessIncidentBatch(t *testing.T) {
    // This test MUST fail until implementation exists
    processor := NewBackgroundIncidentProcessor(cache, openaiClient)
    
    incidents := []interface{}{
        &CaltransIncident{Description: "Test incident 1"},
        &CaltransIncident{Description: "Test incident 2"},
    }
    
    err := processor.ProcessIncidentBatch(context.Background(), incidents)
    require.NoError(t, err)
    
    // Verify incidents were queued for background processing
    stats, err := processor.GetProcessingStats(context.Background())
    require.NoError(t, err)
    require.Equal(t, int64(2), stats.QueuedIncidents)
}
```

### Integration Tests (Real Dependencies)

```go
// Test end-to-end flow with real OpenAI API
func TestAsyncAlertEnhancer_RealOpenAI(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Use real OpenAI client
    enhancer := NewAsyncAlertEnhancer(realOpenAIClient, cache)
    
    incident := &CaltransIncident{
        Description: "I-80 WESTBOUND CHAIN CONTROLS REQUIRED",
    }
    
    // First call should trigger background processing
    result, fromCache, err := enhancer.GetEnhancedAlert(context.Background(), incident)
    require.NoError(t, err)
    require.False(t, fromCache) // First call not cached
    
    // Wait for background processing
    time.Sleep(5 * time.Second)
    
    // Second call should be served from cache
    result2, fromCache2, err := enhancer.GetEnhancedAlert(context.Background(), incident)
    require.NoError(t, err)
    require.True(t, fromCache2) // Second call from cache
}
```

### Performance Validation

```go
// Test <200ms response time requirement
func TestAPIResponseTime_Under200ms(t *testing.T) {
    server := setupTestServer()
    
    start := time.Now()
    resp, err := http.Get(server.URL + "/api/v1/routes/i80")
    duration := time.Since(start)
    
    require.NoError(t, err)
    require.Equal(t, http.StatusOK, resp.StatusCode)
    require.Less(t, duration.Milliseconds(), int64(200), 
        "API response time must be under 200ms")
}
```

## Monitoring and Metrics

The system provides comprehensive metrics for monitoring cache effectiveness and background processing performance:

```go
// Check cache performance
metrics, err := cache.GetCacheMetrics(ctx)
fmt.Printf("Cache hit rate: %.2f%%\n", metrics.CacheHitRate*100)
fmt.Printf("Avg response time: %.1fms\n", metrics.AvgResponseTimeMs["cached"])

// Check background processing status
stats, err := processor.GetProcessingStats(ctx)
fmt.Printf("OpenAI calls saved: %d\n", stats.OpenAICallsSaved)
fmt.Printf("Estimated cost savings: $%.2f\n", stats.CostSavingsEstimate)

// Health check for <200ms requirement
status, err := enhancer.GetEnhancementStatus(ctx)
fmt.Printf("System healthy (P95 < 200ms): %v\n", status.IsHealthy)
```

## Expected Performance Improvements

### **Before Implementation**:
- Every incident processed through OpenAI on every API request
- API response times: 2-5 seconds (waiting for OpenAI)
- OpenAI API costs: $X per day for repeat processing

### **After Implementation**:
- Cached incidents served immediately: <200ms
- Background processing handles OpenAI calls asynchronously
- 80-90% reduction in OpenAI API costs for repeat incidents
- Graceful degradation: serves raw data if processing fails

## Deployment Strategy

1. **Phase 1**: Deploy with background processing disabled to test caching infrastructure
2. **Phase 2**: Enable background processing with conservative rate limits
3. **Phase 3**: Tune performance based on metrics and gradually increase processing concurrency
4. **Phase 4**: Enable prefetching for common incidents based on historical patterns

This implementation maintains the existing API interface while dramatically improving performance through intelligent background processing and content-based caching.