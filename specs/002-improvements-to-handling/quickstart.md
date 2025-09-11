# Quickstart: Smart Route-Relevant Alert Filtering

**Feature**: 002-improvements-to-handling | **Date**: 2025-09-11  
**Purpose**: End-to-end testing scenarios derived from user stories

## Prerequisites

### API Keys Required
```bash
# Set required environment variables
export PF__ROADS__GOOGLE_ROUTES__API_KEY="your-google-api-key"
export PF__WEATHER__OPENWEATHER_API_KEY="your-openweather-api-key"  
export PF__ALERTS__OPENAI_API_KEY="your-openai-api-key"  # NEW
```

### Test Data Setup
```bash
# Ensure timestamped KML test data exists
make fetch-test-data

# Build all components including new libraries
make build
```

## User Story Validation

### Story 1: On Route Lane Closure Detection
**Scenario**: Lane closure directly on Highway 4 between Angels Camp and Murphys

```bash
# Step 1: Start server with enhanced alert processing
make run-bg

# Step 2: Query route with known on-route closure
curl -s "http://localhost:8080/api/v1/roads/hwy4-angels-murphys" | \
  jq '.road.alerts[] | select(.classification == "ON_ROUTE")'

# Expected: At least one alert with:
# - classification: "ON_ROUTE"  
# - enhanced_description with human-readable text
# - condensed_summary in format: "Hwy 4 – Location: Description (Time)"
```

### Story 2: Nearby Incident Classification  
**Scenario**: Incident on side street in Murphys area

```bash
# Query same route, check for nearby alerts
curl -s "http://localhost:8080/api/v1/roads/hwy4-angels-murphys" | \
  jq '.road.alerts[] | select(.classification == "NEARBY")'

# Expected: Alerts with:
# - classification: "NEARBY"
# - Clear indication in enhanced_description that it doesn't block main route
# - Lower priority in response ordering (after ON_ROUTE alerts)
```

### Story 3: Distant Alert Filtering
**Scenario**: Incidents 50+ miles away should not appear

```bash
# Query route and count total alerts
ALERT_COUNT=$(curl -s "http://localhost:8080/api/v1/roads/hwy4-angels-murphys" | \
  jq '.road.alerts | length')

# Expected: Reasonable number of alerts (< 20), not hundreds
# Distant alerts should be filtered out automatically
echo "Alert count for route: $ALERT_COUNT"
```

### Story 4: Human-Readable Descriptions
**Scenario**: Alert descriptions understandable without technical knowledge

```bash
# Check enhancement quality
curl -s "http://localhost:8080/api/v1/roads/hwy4-angels-murphys" | \
  jq '.road.alerts[0] | {
    original: .original_description,
    enhanced: .enhanced_description,
    summary: .condensed_summary
  }'

# Expected:
# - original_description: Raw Caltrans text with technical jargon
# - enhanced_description: Structured JSON with readable fields
# - condensed_summary: Brief, clear format for mobile display
```

## Integration Test Scenarios

### Test 1: Library Integration
**Purpose**: Verify all three libraries work together

```bash
# Test geo-utils CLI
./bin/test-geo-utils polyline-distance \
  --lat 38.1391 --lng -120.4561 \
  --polyline "_p~iF~ps|U_ulLnnqC_mqNvxq`@"

# Test alert-enhancer CLI  
./bin/test-alert-enhancer enhance-alert \
  --description "Rte 4 EB of MM 31 - VEHICLE IN DITCH, EMS ENRT" \
  --location "Highway 4"

# Test route-matcher CLI
./bin/test-route-matcher classify-alert \
  --alert-json test_alert.json \
  --routes-json test_routes.json
```

### Test 2: OpenAI Integration
**Purpose**: Verify AI enhancement works correctly

```bash
# Test OpenAI connection
curl -X POST "http://localhost:8080/api/v1/test/openai-health"

# Expected: 200 OK with rate limit information
# Should show available requests and tokens remaining
```

### Test 3: Performance Validation
**Purpose**: Ensure response times meet requirements

```bash
# Measure enhanced API response time  
time curl -s "http://localhost:8080/api/v1/roads" > /dev/null

# Expected: < 2 seconds total response time
# Most delay should be in initial cache warming, not per-request processing
```

### Test 4: Cache Behavior  
**Purpose**: Verify caching reduces OpenAI API calls

```bash
# Clear cache and make first request
curl -X POST "http://localhost:8080/api/v1/admin/clear-cache"
curl -s "http://localhost:8080/api/v1/roads" > /dev/null

# Make second request immediately  
curl -s "http://localhost:8080/api/v1/roads" > /dev/null

# Check processing metrics
curl -s "http://localhost:8080/api/v1/roads/metrics" | \
  jq '.enhancement_failures'

# Expected: Second request should be much faster
# No additional OpenAI calls should be made for cached results
```

## Error Handling Validation

### Test 1: OpenAI API Failure
**Purpose**: Verify graceful degradation when AI enhancement fails

```bash
# Temporarily set invalid API key
export PF__ALERTS__OPENAI_API_KEY="invalid-key"
make stop && make run-bg

# Query API
curl -s "http://localhost:8080/api/v1/roads/hwy4-angels-murphys" | \
  jq '.road.alerts[0]'

# Expected: 
# - Request still succeeds (no 500 errors)
# - enhanced_description falls back to original_description
# - condensed_summary is generated from original text
```

### Test 2: Invalid Route Geometry
**Purpose**: Handle corrupted or missing polyline data

```bash
# Test with invalid polyline
./bin/test-geo-utils decode-polyline --polyline "invalid_polyline_data"

# Expected: Clear error message about invalid encoding
# System should not crash or return incorrect distances
```

### Test 3: Geographic Edge Cases
**Purpose**: Handle alerts at route boundaries

```bash
# Test classification of alert exactly at route endpoint
./bin/test-route-matcher classify-alert \
  --alert-json endpoint_alert.json \
  --routes-json routes.json

# Expected: Alert classified as ON_ROUTE if at exact endpoint
# Clear handling of boundary conditions
```

## Monitoring Validation

### Metrics Collection
```bash
# Get processing metrics
curl -s "http://localhost:8080/api/v1/roads/metrics" | jq '.'

# Expected fields:
# - total_raw_alerts: Number from Caltrans feeds
# - filtered_alerts: After distance filtering  
# - enhanced_alerts: Successfully processed by AI
# - enhancement_failures: AI processing failures
# - avg_processing_time_ms: Performance metric
```

### Health Checks
```bash
# Check all service health
curl -s "http://localhost:8080/health" | jq '.'

# Expected: All services (Google, OpenAI, Caltrans) show healthy status
# Rate limit information included for external APIs
```

## Cleanup

```bash
# Stop test server
make stop

# Restore environment  
export PF__ALERTS__OPENAI_API_KEY="your-real-api-key"

# Clean up test files
rm -f test_alert.json test_routes.json
```

## Success Criteria

### Functional Requirements Met
- ✅ FR-001: Alerts filtered to relevant routes only
- ✅ FR-002: Clear ON_ROUTE vs NEARBY classification  
- ✅ FR-003: Human-readable descriptions via AI
- ✅ FR-004: Visual distinction between alert types
- ✅ FR-005: ON_ROUTE alerts prioritized in response
- ✅ FR-006: Distant alerts excluded (>10 miles)
- ✅ FR-007: Descriptions understandable to general public
- ✅ FR-008: Multi-route incidents appear in all affected routes

### Performance Requirements Met
- ✅ API response time < 2 seconds  
- ✅ Cache refresh within 5-minute intervals
- ✅ AI enhancement success rate > 95%
- ✅ Distance calculations accurate within 1% margin

### Quality Requirements Met
- ✅ Graceful degradation when AI unavailable
- ✅ All libraries follow TDD principles
- ✅ Comprehensive error handling and logging
- ✅ Processing metrics available for monitoring