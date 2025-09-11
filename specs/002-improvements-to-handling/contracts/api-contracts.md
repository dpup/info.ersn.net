# API Contracts: Smart Route-Relevant Alert Filtering

**Feature**: 002-improvements-to-handling | **Date**: 2025-09-11  
**Purpose**: Define interfaces and API contracts for implementation

## Updated gRPC API (In-Place Replacement)

### Modified Message Types

```protobuf
// Updated RoadAlert with classification and AI-processed description
message RoadAlert {
  string id = 1;
  AlertType type = 2;
  AlertSeverity severity = 3;
  AlertClassification classification = 4;  // NEW: On route vs nearby
  string title = 5;
  string original_description = 6;         // NEW: Raw Caltrans description
  string description = 7;                  // UPDATED: Now AI-processed description
  string condensed_summary = 8;            // NEW: Short format for mobile
  google.protobuf.Timestamp start_time = 9;
  google.protobuf.Timestamp end_time = 10;
  google.protobuf.Timestamp last_updated = 11;
  repeated string affected_segments = 12;
  Coordinates location = 13;
  map<string, string> metadata = 14;      // NEW: Dynamic key-value pairs
}

enum AlertClassification {
  ALERT_CLASSIFICATION_UNSPECIFIED = 0;
  ON_ROUTE = 1;      // Directly affects route path
  NEARBY = 2;        // In surrounding area but not blocking route
}
```

### Updated Service Methods (No Name Changes)

```protobuf
service RoadsService {
  rpc ListRoads(ListRoadsRequest) returns (ListRoadsResponse);     // Same endpoints
  rpc GetRoad(GetRoadRequest) returns (GetRoadResponse);           // Same endpoints
  rpc GetProcessingMetrics(GetProcessingMetricsRequest) returns (ProcessingMetrics); // NEW
}
```

## Library Interface Contracts

### 1. Geo-Utils Library (`internal/lib/geo/`)

**Core Interface**:
```go
type GeoUtils interface {
    // Calculate great-circle distance between two points in meters
    PointToPoint(p1, p2 Point) (float64, error)
    
    // Calculate minimum distance from point to polyline in meters
    PointToPolyline(point Point, polyline Polyline) (float64, error)
    
    // Check if two polylines overlap (for closure vs route matching)
    PolylinesOverlap(polyline1, polyline2 Polyline, thresholdMeters float64) (bool, []OverlapSegment, error)
    
    // Calculate percentage of polyline1 that overlaps with polyline2
    PolylineOverlapPercentage(polyline1, polyline2 Polyline, thresholdMeters float64) (float64, error)
    
    // Decode Google polyline string to point sequence
    DecodePolyline(encoded string) ([]Point, error)
    
    // Find closest point on polyline to given point
    ClosestPointOnPolyline(point Point, polyline Polyline) (Point, error)
}

type OverlapSegment struct {
    StartPoint Point    `json:"start_point"`
    EndPoint   Point    `json:"end_point"`
    Length     float64  `json:"length_meters"`
}

type Point struct {
    Latitude  float64 `json:"lat"`
    Longitude float64 `json:"lng"`
}

type Polyline struct {
    EncodedPolyline string  `json:"encoded_polyline"`
    Points          []Point `json:"points"`
}
```

**CLI Commands**:
- `test-geo-utils point-distance --lat1 X --lng1 Y --lat2 X --lng2 Y`
- `test-geo-utils polyline-distance --lat X --lng Y --polyline "encoded"`
- `test-geo-utils polyline-overlap --polyline1 "encoded1" --polyline2 "encoded2" --threshold 50`
- `test-geo-utils decode-polyline --polyline "encoded_string"`

### 2. Alert-Enhancer Library (`internal/lib/alerts/`)

**Core Interface**:
```go
type AlertEnhancer interface {
    // Enhance single alert with AI processing
    EnhanceAlert(ctx context.Context, raw RawAlert) (EnhancedAlert, error)
    
    // Generate condensed summary format
    GenerateCondensedSummary(ctx context.Context, enhanced StructuredDescription) (string, error)
    
    // Health check for AI service
    HealthCheck(ctx context.Context) error
}

type StructuredDescription struct {
    TimeReported   string            `json:"time_reported,omitempty"`
    Details        string            `json:"details"`
    Location       string            `json:"location"`
    LastUpdate     string            `json:"last_update,omitempty"`
    Impact         string            `json:"impact"`
    Duration       string            `json:"duration"`
    AdditionalInfo map[string]string `json:"additional_info,omitempty"`
}
```

**OpenAI Prompt Design**:
```json
{
  "system_message": "You are a traffic incident analyst specializing in converting technical road incident reports into clear, public-friendly descriptions. Extract structured information and generate readable summaries for travelers.",
  
  "user_template": "Parse this Caltrans traffic incident report and return structured JSON:\n\nOriginal: {raw_description}\nLocation Context: {location_context}\n\nReturn JSON with these fields:\n- time_reported: Parse any timestamps (ISO format or null)\n- details: Core incident info without jargon\n- location: Human-readable location \n- last_update: Most recent update time (ISO or null)\n- impact: traffic impact level (none/light/moderate/severe)\n- duration: expected duration (unknown/< 1 hour/several hours/ongoing)\n- additional_info: Key-value object for specific details like visibility, lanes affected, etc.\n\nAlso generate:\n- condensed_summary: Single line format '{Highway} – {Location}: {Brief description} ({time})' under 150 chars",
  
  "output_schema": {
    "type": "object",
    "properties": {
      "time_reported": {"type": ["string", "null"]},
      "details": {"type": "string"},
      "location": {"type": "string"}, 
      "last_update": {"type": ["string", "null"]},
      "impact": {"enum": ["none", "light", "moderate", "severe"]},
      "duration": {"enum": ["unknown", "< 1 hour", "several hours", "ongoing"]},
      "additional_info": {"type": "object"},
      "condensed_summary": {"type": "string", "maxLength": 150}
    },
    "required": ["details", "location", "impact", "duration", "condensed_summary"]
  }
}
```

**CLI Commands**:
- `test-alert-enhancer enhance-alert --description "raw text" --location "Hwy 4"`
- `test-alert-enhancer test-connection --api-key "sk-..." --model "gpt-3.5-turbo"`
- `test-alert-enhancer generate-summary --enhanced-json "data.json"`
- `test-alert-enhancer test-prompt --raw-file sample_incidents.txt`

### 3. Route-Matcher Library (`internal/lib/routing/`)

**Core Interface**:
```go
type RouteMatcher interface {
    // Classify single alert against all routes
    ClassifyAlert(ctx context.Context, alert UnclassifiedAlert, routes []Route) (ClassifiedAlert, error)
    
    // Get alerts for specific route
    GetRouteAlerts(ctx context.Context, routeID string, alerts []ClassifiedAlert) ([]ClassifiedAlert, error)
    
    // Update route geometry when Google Routes data refreshes
    UpdateRouteGeometry(ctx context.Context, routeID string, newPolyline Polyline) error
}

type AlertClassification string
const (
    OnRoute AlertClassification = "on_route" // < 100m from polyline
    Nearby  AlertClassification = "nearby"   // < configured threshold  
    Distant AlertClassification = "distant"  // > threshold (filtered out)
)

type ClassifiedAlert struct {
    UnclassifiedAlert
    Classification  AlertClassification `json:"classification"`
    RouteIDs       []string            `json:"route_ids"`
    DistanceToRoute float64             `json:"distance_to_route"`
}
```

**CLI Commands**:
- `test-route-matcher classify-alert --alert-json alert.json --routes-json routes.json`
- `test-route-matcher test-distance --route-id hwy4-angels-murphys --lat X --lng Y`
- `test-route-matcher validate-route --route-json route.json`

## Integration Points

### Existing Service Modifications

**Caltrans Parser Enhancement** (`internal/clients/caltrans/parser.go`):
```go
// Add route-based classification to existing geographic filtering
func (p *FeedParser) ParseWithRouteClassification(
    ctx context.Context, 
    routes []Route, 
    radiusMeters float64
) ([]ClassifiedAlert, error)
```

**Roads Service Enhancement** (`internal/services/roads.go`):
```go
// Integrate all three libraries in alert processing pipeline
func (s *RoadsService) processEnhancedAlerts(
    ctx context.Context,
    incidents []CaltransIncident,
    routes []Route
) ([]*api.EnhancedRoadAlert, error)
```

### Configuration Extensions

**New Config Fields** (`prefab.yaml`):
```yaml
alerts:
  openai_api_key: ""  # Set via PF__ALERTS__OPENAI_API_KEY
  enhancement_enabled: true
  enhancement_timeout: "10s"
  fallback_on_failure: true
  
roads:
  alert_distance_threshold_km: 10  # Configurable "nearby" threshold
  on_route_threshold_meters: 100   # "On route" classification
```

## Error Handling Contracts

### Standard Error Types
- `ErrInvalidCoordinates`: Invalid lat/lng values
- `ErrPolylineDecoding`: Invalid Google polyline format
- `ErrOpenAITimeout`: AI enhancement timed out
- `ErrOpenAIRateLimit`: API rate limit exceeded
- `ErrClassificationFailed`: Route matching failed

### Graceful Degradation
- AI enhancement failure → use original description
- Route classification failure → default to "nearby"
- Distance calculation error → exclude alert
- All failures logged with context for debugging

## Testing Contracts

### Contract Test Requirements
1. **Geo-Utils**: Distance calculations accurate within 1% margin
2. **Alert-Enhancer**: AI enhancement success rate > 95% with retries
3. **Route-Matcher**: Classification consistency across multiple runs

### Integration Test Requirements
1. **End-to-End**: Quickstart scenarios must pass
2. **Performance**: API response time < 2 seconds
3. **Reliability**: Graceful handling of all external API failures

This documentation-based approach avoids IDE confusion while clearly defining the contracts for implementation.