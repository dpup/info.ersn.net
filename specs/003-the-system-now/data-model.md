# Data Model: Background Processing for Incident Content Caching

**Feature**: Background Processing with Content-Based Caching  
**Date**: 2025-09-12  
**Status**: Complete

## Entity Overview

The background processing system introduces content-based incident identification and out-of-band processing to achieve <200ms API response times while extending existing cache infrastructure.

## Core Entities

### **1. IncidentContentHash**
Provides deterministic content-based identification for incident deduplication across feed refreshes.

**Attributes**:
- `ContentHash`: `string` - SHA-256 hash of normalized incident content
- `NormalizedText`: `string` - Cleaned incident description for consistent hashing
- `LocationKey`: `string` - Geographic coordinates with appropriate precision
- `IncidentCategory`: `string` - Incident type (closure, chain_control, chp_incident)
- `FirstSeenAt`: `time.Time` - When this content hash was first generated

**Key Properties**:
- Deterministic hashing enables duplicate detection across feed refreshes
- Text normalization handles minor variations in incident descriptions
- Location precision balances accuracy with duplicate detection
- Immutable content hash for consistent caching behavior

**Validation Rules**:
- ContentHash must be valid 64-character SHA-256 hex string
- LocationKey must follow "lat_lng_precision" format (e.g., "39.123_-120.567_3")
- IncidentCategory must be one of: "closure", "chain_control", "chp_incident"
- FirstSeenAt cannot be in the future

### **2. ProcessedIncidentCache**
Stores background-processed incident data with expiration based on feed presence.

**Attributes**:
- `ContentHash`: `IncidentContentHash` - Unique incident identifier
- `Stage`: `ProcessingStage` - Level of processing applied (RAW_KML, ROUTE_FILTERED, OPENAI_ENHANCED)
- `OriginalIncident`: `interface{}` - Raw Caltrans incident data
- `ProcessedData`: `interface{}` - Enhanced/classified result from background processing
- `LastSeenInFeed`: `time.Time` - Most recent occurrence in Caltrans feeds
- `CacheExpiresAt`: `time.Time` - When cache entry expires (1 hour after last seen)
- `ServeCount`: `int64` - Number of times this cached result was served to API requests
- `ProcessingDuration`: `time.Duration` - How long background processing took

**Key Properties**:
- Feed-aware expiration: expires 1 hour after incident disappears from feeds
- Tracks processing cost savings for monitoring
- Supports incremental processing stages
- Optimized for fast API serving (<200ms requirement)

**Validation Rules**:
- Stage must be valid ProcessingStage enum value
- LastSeenInFeed must be updated when incident appears in feed refresh
- CacheExpiresAt must be LastSeenInFeed + 1 hour
- ProcessingDuration must be positive for OPENAI_ENHANCED stage

### **3. ProcessingStage (Enum)**
Defines the different levels of incident processing for background processing organization.

**Values**:
- `RAW_KML` - Parsed KML data with no additional processing
- `ROUTE_FILTERED` - Geographic filtering applied for specific routes
- `OPENAI_ENHANCED` - OpenAI processing complete (most expensive)

**Usage**:
- Organizes cache entries by processing complexity
- Enables selective background processing based on priority
- Supports performance analysis by processing stage

### **4. BackgroundProcessingConfig**
Configuration for out-of-band incident processing to achieve <200ms API responses.

**Attributes**:
- `ProcessingIntervalMinutes`: `int` - How often to check for new incidents  
- `MaxConcurrentOpenAI`: `int` - Limit concurrent OpenAI API calls to avoid rate limits
- `CacheTTLHours`: `int` - Hours to keep processed incidents after last seen in feeds
- `PrefetchEnabled`: `bool` - Whether to proactively process common/seasonal incidents
- `OpenAITimeoutSeconds`: `int` - Timeout for individual OpenAI API calls

**Default Values**:
```go
StoreConfig{
    ProcessingIntervalMinutes:  5,   // Check for new incidents every 5 minutes
    MaxConcurrentOpenAI:        3,   // Conservative to avoid OpenAI rate limits
    CacheTTLHours:             1,    // 1 hour after incident disappears from feeds
    PrefetchEnabled:           true, // Proactive processing recommended
    OpenAITimeoutSeconds:      30,   // Reasonable timeout for OpenAI calls
}
```

**Key Properties**:
- Configurable per deployment environment
- Supports feature flags for gradual rollout
- Performance tuning capabilities
- Backward compatible with existing cache configuration

## Relationships and Data Flow

### **Entity Relationships**
```
IncidentFingerprint (1) ←→ (1) CachedIncidentEntry
CaltransIncident (1) ←→ (1) CachedIncidentEntry
ProcessingLevel (1) ←→ (*) CachedIncidentEntry
IntelligentCacheConfig (1) ←→ (*) ProcessingLevel
```

### **Data Flow Stages**
1. **Raw Incident** (`CaltransIncident`) → **Fingerprint Generation** → `IncidentFingerprint`
2. **Cache Lookup** by Fingerprint → Hit: Return `CachedIncidentEntry` / Miss: Continue processing  
3. **Processing** (Filter/Classify/Enhance) → **Cache Store** → `CachedIncidentEntry`
4. **Background Refresh** → **TTL Check** → **Proactive Refresh** or **Expiration**

### **State Transitions**
```
Raw Incident → Fingerprinted → Cached → Refreshed → Expired
                    ↓              ↓         ↑
                Cache Miss → Processing → Cache Store
```

## Integration with Existing Models

### **Extends Existing Types**
- `cache.CacheEntry` - Base caching functionality preserved
- `CaltransIncident` - Raw incident data structure unchanged  
- `RawAlert`, `ClassifiedAlert`, `EnhancedAlert` - Processing pipeline unchanged

### **Configuration Integration**
- Adds `Store` section to existing `Config` struct
- Uses existing `koanf` struct tags for environment variable support
- Follows `PF__STORE__` prefix pattern

### **Service Integration**
- `RoadsService` - Gains content-based caching without interface changes
- `AlertEnhancer` - Wrapped with `CachedAlertEnhancer` decorator
- `RouteMatcher` - Wrapped with `CachedRouteMatcher` decorator

## Memory and Performance Considerations

### **Memory Usage**
- **Fingerprint**: ~200 bytes per incident
- **Cached Entry**: ~1-5KB per cached incident (varies by processing level)
- **Estimated Total**: 100-500KB for typical deployment (50-100 active incidents)

### **Performance Characteristics**
- **Hash Generation**: O(1) per incident, ~1ms
- **Cache Lookup**: O(1) hash table access, ~0.1ms
- **Cache Store**: O(1) insertion, ~0.1ms
- **Background Refresh**: Async, non-blocking user requests

### **Scalability**
- Linear memory growth with incident count
- Bounded by TTL-based expiration
- Optional LRU eviction for memory-constrained environments

## Error Handling and Edge Cases

### **Fingerprint Conflicts**
- SHA-256 collision probability: negligible for practical purposes
- Include location coordinates in hash to reduce false positives
- Log suspicious duplicate incidents for investigation

### **Cache Consistency**
- Thread-safe operations using existing cache synchronization
- Atomic read-modify-write for hit count updates
- Background refresh coordination to prevent race conditions

### **Memory Pressure**
- Optional size-based eviction (configurable limits)
- Metrics and alerting for memory usage monitoring
- Graceful degradation when cache disabled

## Testing Validation

### **Unit Test Coverage**
- Fingerprint generation with various incident formats
- Cache entry lifecycle (create, update, expire)
- Configuration validation and defaults
- Thread safety of concurrent operations

### **Integration Test Scenarios**
- End-to-end incident processing with cache hits/misses
- Cache warming and background refresh cycles
- Performance benchmarks for cache effectiveness
- Memory usage patterns under load

This data model design maintains compatibility with existing infrastructure while providing the foundation for content-based, cost-effective caching of OpenAI-enhanced incident data.