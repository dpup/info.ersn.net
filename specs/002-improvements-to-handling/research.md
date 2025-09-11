# Research: Smart Route-Relevant Alert Filtering

**Feature**: 002-improvements-to-handling | **Date**: 2025-09-11  
**Input**: Technical unknowns and integration patterns from plan.md

## Research Tasks

### 1. Google Routes API Polyline Integration
**Task**: Research how to extract and use polyline data from existing Google Routes API calls for geo-spatial comparisons.

**Decision**: Use Google's `polyline.encoded` field from existing Routes API responses
- Decode polylines using standard algorithm (groups of lat/lng coordinates)
- Store decoded polylines in route cache alongside existing data
- Polylines provide detailed route geometry for precise distance calculations

**Rationale**: 
- Already have Google Routes integration, minimal additional API calls
- Polylines provide accurate route geometry vs simple origin/destination points
- Standard encoding/decoding libraries available

**Alternatives considered**:
- Using only origin/destination points: Less accurate for winding routes
- Third-party mapping services: Unnecessary complexity and additional cost

### 2. Geo-spatial Distance and Proximity Algorithms
**Task**: Find best practices for point-to-polyline distance calculations and fuzzy geo-matching.

**Decision**: Implement point-to-polyline distance using perpendicular distance algorithm
- For each incident point, calculate minimum distance to any segment of route polyline
- Use Haversine formula for great-circle distances between points
- Implement configurable tolerance buffers (default 10 miles)

**Rationale**:
- Perpendicular distance provides accurate "closest point on route" calculation
- Handles curved/winding routes better than simple radius checks
- Configurable thresholds allow tuning for different route types

**Alternatives considered**:
- Simple radius checks from waypoints: Inaccurate for long route segments
- Complex GIS libraries: Overkill for our use case, adds dependencies

### 2.5. Polyline-to-Polyline Comparison for Closures
**Task**: Research algorithms for determining if Caltrans closure polylines overlap with route polylines.

**Decision**: Implement segment-based overlap detection with configurable buffer
- For each segment in closure polyline, find minimum distance to any segment in route polyline
- Consider segments "overlapping" if minimum distance < threshold (default 50 meters)
- Calculate overlap percentage: (overlapping_length / total_route_length) * 100
- Classify as ON_ROUTE if overlap percentage > 10%

**Rationale**: 
- Handles complex closures that span multiple route segments
- Buffer accounts for GPS accuracy and road width variations
- Percentage threshold prevents tiny overlaps from triggering false positives

**Alternatives considered**:
- Point-in-polygon approaches: Too complex for linear road features
- Exact geometric intersection: Too strict, misses parallel closures that affect traffic

### 3. OpenAI API Integration for Description Enhancement  
**Task**: Research OpenAI API best practices for structured incident description processing.

**Decision**: Use OpenAI GPT-3.5-turbo with structured JSON output mode
- Send raw Caltrans description as prompt with structured output schema
- Request standardized fields: time_reported, details, location, last_update
- Allow additional arbitrary key/value pairs for specialized info (visibility, etc.)
- Generate condensed alert format for display

**Rationale**:
- GPT-3.5-turbo offers good quality/cost balance for this task
- JSON output mode ensures parseable responses
- Structured schema provides consistency while preserving flexibility

**Alternatives considered**:
- GPT-4: Higher cost without significant benefit for this structured task
- Local NLP models: Complexity overhead, quality concerns
- Rule-based parsing: Too brittle for varied Caltrans description formats

### 4. Data Model Integration Patterns
**Task**: Research patterns for mapping Google and Caltrans data to common structure.

**Decision**: Implement common `GeoAlert` interface with source-specific adapters
- Define standard alert structure: location, classification, description, metadata
- Create adapters: `CaltransToGeoAlert`, `GoogleToGeoAlert` (for future route hazards)
- Use strategy pattern for different alert enhancement approaches

**Rationale**:
- Clean separation of concerns between data sources and processing logic
- Extensible for future data sources (weather alerts, user reports, etc.)
- Testable adapters for each data source independently

**Alternatives considered**:
- Direct coupling: Harder to test and extend
- Complex ORM-style mapping: Overkill for in-memory processing

### 5. Caching Strategy for Enhanced Alerts
**Task**: Determine optimal caching approach for processed/enhanced alerts.

**Decision**: Two-tier caching strategy
- Tier 1: Raw Caltrans data (existing 5-minute refresh)
- Tier 2: Enhanced alerts with route classification (15-minute refresh)
- Cache enhanced alerts by route ID to avoid re-processing

**Rationale**:
- Minimizes OpenAI API calls while maintaining reasonable freshness
- Route-specific caches optimize for typical access patterns
- Longer cache for enhanced data accounts for processing time

**Alternatives considered**:
- Single-tier caching: Would require re-enhancement on every raw data refresh
- No caching of enhanced data: Excessive OpenAI API costs and latency

## Integration Points

### Existing Codebase (In-Place Updates)
- **Current**: `internal/clients/caltrans/parser.go` - Geographic filtering with haversine distance
- **Update**: Replace simple distance filtering with polyline-aware classification
- **Current**: `internal/services/roads.go` - Alert processing and API response building  
- **Update**: Replace alert processing pipeline with AI-enhanced version
- **Current**: `api/v1/roads.proto` - Basic RoadAlert message
- **Update**: Add classification, original_description, condensed_summary fields (no "Enhanced" prefix)

### New Libraries Structure
```
internal/
├── lib/
│   ├── geo/              # geo-utils library
│   │   ├── polyline.go   # Polyline encoding/decoding
│   │   ├── distance.go   # Point-to-polyline distance calculations
│   │   └── cli/          # test-geo-utils CLI
│   ├── alerts/           # alert-enhancer library
│   │   ├── enhancer.go   # OpenAI integration
│   │   ├── schema.go     # Structured output schemas
│   │   └── cli/          # test-alert-enhancer CLI
│   └── routing/          # route-matcher library
│       ├── matcher.go    # Alert classification logic
│       ├── common.go     # GeoAlert interface and adapters
│       └── cli/          # test-route-matcher CLI
```

### API Dependencies
- **Google Routes API**: No additional calls needed, use existing polyline data
- **OpenAI API**: New integration, requires API key configuration
- **Caltrans KML**: Continue using existing feeds with enhanced processing

## Risk Assessment

### Technical Risks
- **OpenAI API Rate Limits**: Mitigated by aggressive caching of enhanced descriptions
- **Google Polyline Complexity**: Standard algorithms available, well-documented
- **Geo-spatial Accuracy**: Start with 10-mile threshold, tune based on real-world testing

### Performance Risks  
- **Enhancement Latency**: 15-minute cache prevents blocking API responses
- **Memory Usage**: Enhanced alerts add ~2KB per incident, manageable for current scale
- **Processing Overhead**: Batch enhancement during cache refresh, not per-request

## Success Criteria
- Enhanced alerts cache within 200ms of raw data refresh
- OpenAI enhancement maintains <95% success rate with retries
- Point-to-polyline distance calculations accurate within 1% margin
- All new libraries follow TDD principles with >90% test coverage