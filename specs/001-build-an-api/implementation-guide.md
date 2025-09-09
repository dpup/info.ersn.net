# Implementation Guide - Live Data API Server

## Cross-Reference Map

This guide provides concrete implementation examples and cross-references between planning documents and code implementation.

### Document Navigation
- **Requirements** → `spec.md` functional requirements (FR-001 through FR-017)
- **Architecture** → `research.md` technical decisions and rationale
- **Data Structures** → `data-model.md` entities and relationships
- **API Contracts** → `contracts/roads.proto`, `contracts/weather.proto`
- **Setup Instructions** → `quickstart.md` build and run procedures
- **Task Sequence** → `plan.md` Phase 2 implementation approach

## API Response Mapping Examples

### Google Routes API → TrafficCondition Proto
```go
// Google Routes API Response:
{
  "routes": [{
    "duration": "450s",
    "distanceMeters": 15240,
    "polyline": {"encodedPolyline": "encoded_string_here"},
    "travelAdvisory": {
      "speedReadingIntervals": [{
        "startPolylinePointIndex": 0,
        "endPolylinePointIndex": 10,
        "speed": "NORMAL"
      }]
    }
  }]
}

// Maps to TrafficCondition proto:
traffic_condition := &api.TrafficCondition{
    RouteId:         routeID,
    DurationSeconds: parseDuration("450s"), // 450
    DistanceMeters:  15240,
    Polyline:        "encoded_string_here",
    SpeedReadings:   convertSpeedReadings(intervals),
    CongestionLevel: deriveCongestionLevel(intervals),
    AverageSpeedMph: calculateSpeed(15240, 450),
}
```

### OpenWeatherMap API → WeatherData Proto
```go
// OpenWeatherMap Current Weather Response:
{
  "coord": {"lat": 33.44, "lon": -94.04},
  "weather": [{"main": "Rain", "description": "moderate rain", "icon": "10d"}],
  "main": {"temp": 298.48, "feels_like": 298.74, "pressure": 1015, "humidity": 64},
  "visibility": 10000,
  "wind": {"speed": 0.62, "deg": 349},
  "clouds": {"all": 100},
  "dt": 1661870592,
  "name": "Seattle"
}

// Maps to WeatherData proto:
weather_data := &api.WeatherData{
    LocationId:         locationID,
    LocationName:       "Seattle",
    Coordinates:        &api.Coordinates{Latitude: 33.44, Longitude: -94.04},
    WeatherMain:        "Rain",
    WeatherDescription: "moderate rain",
    WeatherIcon:        "10d",
    TemperatureKelvin:  298.48,
    TemperatureF:       kelvinToFahrenheit(298.48),
    FeelsLikeF:         kelvinToFahrenheit(298.74),
    HumidityPercent:    64,
    PressureMb:         1015,
    VisibilityMeters:   10000,
    WindSpeedMs:        0.62,
    WindDirectionDeg:   349,
    CloudCoverPercent:  100,
    DataTimestamp:      1661870592,
}
```

### Caltrans KML → CaltransIncident Data Model
```go
// Caltrans KML Structure:
<Placemark>
  <name>US-395 at Conway Summit</name>
  <description><![CDATA[<div>Chain Control in Effect</div>]]></description>
  <styleUrl>#full-closure</styleUrl>
  <Point><coordinates>-119.123,38.456,0</coordinates></Point>
</Placemark>

// Maps to CaltransIncident:
incident := &CaltransIncident{
    FeedType:        CHAIN_CONTROL,
    Name:           "US-395 at Conway Summit",
    DescriptionHtml: "<div>Chain Control in Effect</div>",
    DescriptionText: extractText(htmlContent),
    StyleUrl:       "#full-closure",
    Coordinates:    &Coordinates{Longitude: -119.123, Latitude: 38.456},
    ParsedStatus:   extractStatus(htmlContent),
    LastFetched:    time.Now(),
}
```

## Implementation Examples

### Foundation Layer

#### 1. Project Setup
**Reference**: `research.md` project structure, `quickstart.md` installation
```bash
# Create project structure (see research.md line 164-200)
mkdir -p {cmd/{server,test-google,test-caltrans,test-weather},internal/{services,clients,parsers,cache,config},api/v1,tests/{contract,integration,unit}}

# Initialize Go module (see quickstart.md line 17)
go mod init github.com/dpup/info.ersn.net/server
```

#### 2. Configuration Implementation
**Reference**: `research.md` configuration structure (lines 112-146), `data-model.md` for validation rules
```go
// internal/config/config.go
type Config struct {
    Server    ServerConfig    `yaml:"server"`
    Routes    RoutesConfig    `yaml:"routes"`
    Weather   WeatherConfig   `yaml:"weather"`
}

// Maps to research.md configuration YAML structure
type RoutesConfig struct {
    GoogleRoutes   GoogleConfig    `yaml:"google_routes"`
    CaltransFeeds  CaltransConfig  `yaml:"caltrans_feeds"`
    MonitoredRoutes []RouteConfig  `yaml:"monitored_routes"`
}
```

### External API Clients

#### 3. Google Routes Client
**Reference**: `research.md` Google Routes API (lines 32-47), `data-model.md` TrafficCondition entity (lines 25-36)
```go
// internal/clients/google/client.go
type Client struct {
    apiKey     string
    httpClient *http.Client
    // Rate limiting and quota management per research.md
}

// ComputeRoutes implements coordinate-based API calls per research.md line 42
func (c *Client) ComputeRoutes(ctx context.Context, origin, destination Coordinates) (*RouteData, error) {
    // POST to /directions/v2:computeRoutes
    // Headers: X-Goog-Api-Key, X-Goog-FieldMask, Content-Type
    // Field mask: routes.duration,routes.distanceMeters,routes.polyline.encodedPolyline,routes.travelAdvisory.speedReadingIntervals
}
```

#### 4. Caltrans KML Parser
**Reference**: `research.md` Caltrans integration (lines 49-67), `data-model.md` Route status fields (lines 11-17)
```go
// internal/clients/caltrans/parser.go
type FeedParser struct {
    kmlParser *kml.KML
    // Geographic filtering for route proximity
}

// ParseChainControls processes https://quickmap.dot.ca.gov/data/cc.kml
func (p *FeedParser) ParseChainControls(ctx context.Context) ([]ChainControlData, error) {
    // Download KML, parse with github.com/twpayne/go-kml
    // Filter by geographic proximity to monitored routes
    // Extract chain control status per data-model.md ChainControlStatus enum
}
```

### Service Implementation

#### 5. gRPC Service Structure
**Reference**: `contracts/roads.proto` service definitions, `data-model.md` Route entity
```go
// internal/services/roads.go
type RoadsService struct {
    googleClient   clients.GoogleRoutesClient
    caltransClient clients.CaltransClient
    cache          cache.Cache
    config         *config.RoutesConfig
    api.UnimplementedRoadsServiceServer
}

// ListRoutes implements the gRPC method defined in contracts/roads.proto line 12-17
func (s *RoadsService) ListRoutes(ctx context.Context, req *api.ListRoutesRequest) (*api.ListRoutesResponse, error) {
    // 1. Check cache for fresh data (see data-model.md Cache Entry)
    // 2. If stale, refresh from external APIs
    // 3. Combine Google Routes traffic + Caltrans status + alerts
    // 4. Return unified Route entities per data-model.md Route structure
}
```

### Testing Implementation

#### 6. Contract Test Example
**Reference**: `contracts/roads.proto` ListRoutes method, TDD requirements in `plan.md` lines 68-73
```go
// tests/contract/roads_test.go
func TestRoadsService_ListRoutes_Contract(t *testing.T) {
    // This test MUST fail initially (RED phase)
    service := &services.RoadsService{} // No implementation yet
    
    req := &api.ListRoutesRequest{}
    resp, err := service.ListRoutes(context.Background(), req)
    
    // Assert contract requirements from contracts/roads.proto
    require.NoError(t, err)
    require.NotNil(t, resp)
    require.NotNil(t, resp.LastUpdated)
    // Test will fail until ListRoutes is implemented
}
```

#### 7. CLI Testing Tool
**Reference**: `research.md` CLI tool features (lines 169-174), `quickstart.md` troubleshooting (lines 255-261)
```go
// cmd/test-google/main.go
func main() {
    var (
        configFile = flag.String("config", "config.yaml", "Configuration file")
        routeID    = flag.String("route-id", "", "Route ID to test")
        verbose    = flag.Bool("verbose", false, "Verbose output")
        format     = flag.String("format", "json", "Output format: json|yaml|table")
    )
    flag.Parse()

    // Load config per research.md configuration structure
    // Test Google Routes API with specific route
    // Output in requested format for debugging
}
```

## Implementation Dependencies

### Task Prerequisites
Each implementation task has clear dependencies based on the sequence in `plan.md` Phase 2:

1. **Foundation Tasks (1-8)**: No dependencies, can start immediately
   - Reference documents: `research.md` project structure, `quickstart.md` setup

2. **External Client Tasks (9-20)**: Requires foundation complete
   - Depends on: Configuration framework, basic project structure
   - Can run in parallel: Google, Caltrans, Weather clients independent
   - References: `research.md` API sections, `data-model.md` entities

3. **Service Tasks (21-28)**: Requires external clients complete
   - Depends on: All external API clients, caching infrastructure
   - References: `contracts/*.proto`, `data-model.md` entities

4. **Integration Tasks (29-35)**: Requires services complete
   - Depends on: All services implemented, contract tests passing
   - References: `quickstart.md` API examples, performance requirements

### Error Resolution Guide
When implementation fails, check these cross-references:
- **Configuration errors** → `research.md` configuration structure + `quickstart.md` environment setup
- **API client failures** → `research.md` external API sections + CLI testing tools
- **Data structure mismatches** → `data-model.md` entities + `contracts/*.proto` definitions
- **gRPC service issues** → `contracts/*.proto` + service implementation examples above
- **Test failures** → Constitutional testing requirements in `plan.md` lines 67-73

## File Organization Map
```
specs/001-build-an-api/
├── spec.md              → Requirements (FR-001 to FR-017)
├── research.md          → Technical decisions and API details
├── data-model.md        → Entity definitions and relationships  
├── contracts/           → gRPC/REST API definitions
├── quickstart.md        → Build, run, and troubleshooting procedures
├── plan.md              → Implementation sequence and cross-references
└── implementation-guide.md → This file - concrete examples and navigation
```

Use this guide as the primary navigation tool when implementing tasks. Each code example includes line number references to the source documentation for easy verification and context.