# Research: Intelligent Caching for OpenAI-Enhanced Caltrans Data

**Feature**: Intelligent Caching Layer  
**Date**: 2025-09-12  
**Status**: Complete

## Research Summary

All technical context has been resolved through comprehensive codebase analysis. No NEEDS CLARIFICATION items remain from the Technical Context section.

## Technology Decisions

### **Decision 1: Hash-Based Incident Identification**
- **What was chosen**: SHA-256 hash of incident description + location + style_url for unique identification
- **Rationale**: 
  - Enables detection of identical incidents across feed refreshes
  - Handles slight text variations (normalize before hashing)
  - Deterministic and collision-resistant
  - Integrates with existing Go crypto/sha256 standard library
- **Alternatives considered**: 
  - Caltrans incident IDs (unreliable, often missing)
  - Exact string matching (too brittle for text variations)
  - Geographic coordinate matching (insufficient - multiple incidents at same location)

### **Decision 2: Multi-Level Cache Architecture**
- **What was chosen**: Four-tier intelligent caching strategy
- **Rationale**: 
  - **Level 1 (AI Enhancement)**: Highest cost savings, longest TTL (24h)
  - **Level 2 (Route Classification)**: Medium performance gain, moderate TTL (1h)
  - **Level 3 (Raw KML Data)**: Reduce external API calls, short TTL (15m)
  - **Level 4 (Filtered Incidents)**: Route-specific optimization, very short TTL (5m)
  - Each level addresses different performance bottlenecks
- **Alternatives considered**: 
  - Single-level caching (insufficient granularity)
  - Database persistence (violates in-memory requirement)
  - Event-driven cache invalidation (adds complexity without clear benefit)

### **Decision 3: Cache Integration Pattern**
- **What was chosen**: Decorator pattern wrapping existing services with cache layer
- **Rationale**: 
  - Preserves existing service interfaces and contracts
  - Maintains separation of concerns (caching vs business logic)
  - Allows graceful fallback when cache fails
  - Easy to test and mock independently
  - Follows existing Go patterns in codebase
- **Alternatives considered**: 
  - Direct cache integration in services (violates SRP)
  - Cache-as-service pattern (adds network overhead for in-memory cache)
  - Aspect-oriented programming (not idiomatic in Go)

### **Decision 4: Cache Key Strategy**
- **What was chosen**: Hierarchical cache keys with content hashing
- **Rationale**: 
  - `alert_enhanced:{content_hash}` for AI-processed alerts
  - `alert_classified:{route_id}:{incident_hash}` for route relevance
  - `incidents_raw:{feed_type}:{timestamp_bucket}` for KML data
  - `incidents_filtered:{route_id}:{timestamp_bucket}` for geographic filtering
  - Enables precise cache invalidation and conflict avoidance
- **Alternatives considered**: 
  - Simple string keys (collision potential)
  - UUID-based keys (not deterministic)
  - Database-style composite keys (over-engineering for in-memory cache)

### **Decision 5: Cache Expiration Strategy**
- **What was chosen**: TTL-based expiration with background refresh
- **Rationale**: 
  - 1 hour after incident disappears from feeds (meets requirement)
  - Background refresh prevents cache stampede
  - Graceful degradation to stale data when OpenAI fails
  - Leverages existing cache.CacheEntry TTL infrastructure
- **Alternatives considered**: 
  - Manual cache invalidation (complex to coordinate)
  - Event-based expiration (requires pub/sub infrastructure)
  - LRU eviction only (doesn't meet freshness requirements)

## Implementation Approach

### **Refactoring Opportunities Identified**

1. **Extract AlertEnhancer Interface**: Currently tightly coupled to OpenAI implementation
   - Create `internal/lib/alerts/enhancer.go` interface
   - Implement `CachedAlertEnhancer` decorator
   - Preserves testability and separation of concerns

2. **Incident Fingerprinting Library**: Create reusable hashing utility
   - `internal/lib/cache/fingerprint.go` for consistent incident identification
   - Handles text normalization and hash generation
   - Supports various fingerprinting strategies

3. **Cache Extension**: Enhance existing cache package with intelligent features
   - Add `CacheWithIntelligence` interface extending base cache
   - Implement content-based keying and TTL strategies
   - Maintain backward compatibility

### **Integration Points**

1. **Service Layer** (`internal/services/roads.go`):
   - Wrap AlertEnhancer with CachedAlertEnhancer decorator
   - Add cache metrics logging
   - Minimal changes to existing pipeline

2. **Configuration** (`internal/config/config.go`):
   - Add `IntelligentCache` section with TTL configurations
   - Environment variable support following `PF__` pattern
   - Backward compatible with existing cache settings

3. **Testing Strategy**:
   - Contract tests for CachedAlertEnhancer interface
   - Integration tests with real OpenAI API
   - Performance benchmarks for cache hit rates

## Dependencies and Best Practices

### **Go Standard Library Usage**
- `crypto/sha256` for content hashing
- `time` package for TTL management
- `sync` package for thread-safe operations

### **External Dependencies**
- No new external dependencies required
- Leverage existing Prefab framework for logging
- Use existing testify for test assertions

### **Performance Considerations**
- In-memory cache optimized for read-heavy workloads
- Hash computation amortized across cache lifetime
- Background refresh prevents blocking user requests
- Cache warming strategies for common routes

## Risk Assessment

### **Low Risk Items**
- Cache implementation (well-established patterns)
- Hash-based identification (deterministic)
- Integration with existing services (decorator pattern)

### **Medium Risk Items**
- OpenAI API rate limiting during cache misses
- Memory usage growth with incident volume
- Cache coherence across service restarts

### **Mitigation Strategies**
- Implement circuit breaker for OpenAI calls
- Add cache size monitoring and alerting
- Document cache warming procedures

## Conclusion

The research phase has successfully resolved all technical unknowns. The intelligent caching architecture can be implemented using established Go patterns while maintaining the project's constitutional principles. The multi-tier approach provides comprehensive optimization without violating separation of concerns or adding architectural complexity.

**Ready for Phase 1**: Design data models, contracts, and implementation quickstart guide.