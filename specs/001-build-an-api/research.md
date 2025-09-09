# Phase 0: Research - Live Data API Server

## Technology Stack Research

### Go + gRPC + gRPC Gateway
**Decision**: Use Go with gRPC services and gRPC Gateway for REST endpoints
**Rationale**: 
- Go provides excellent concurrency for background data fetching
- gRPC offers type-safe service definitions with protobuf
- gRPC Gateway automatically generates REST endpoints from proto definitions
- Strong ecosystem for HTTP APIs and external service integrations

**Alternatives considered**:
- Node.js/Express: Less type safety, more complex for concurrent background tasks
- Python/FastAPI: Slower performance, less suitable for low-latency caching scenarios

### Prefab Framework Integration  
**Decision**: Use github.com/dpup/prefab for server orchestration and configuration
**Rationale**:
- Provides sensible defaults for gRPC server setup
- Built-in configuration management (YAML + env vars + functional options)
- Plugin-based architecture allows clean separation of concerns
- Built-in CORS support for static website integration
- Reduces boilerplate for gRPC Gateway setup

**Alternatives considered**:
- Raw gRPC-Go: More boilerplate, manual configuration management
- Chi/Gin HTTP framework: Would lose gRPC benefits, more REST-only approach

### External API Integration Patterns

### Google Routes API
**Decision**: Use Google Routes API v2 for real-time traffic and routing data with lat/lon coordinates
**Rationale**:
- Provides detailed traffic conditions, speed readings, and route polylines
- Supports origin/destination coordinate-based queries
- Returns granular speed data with `TRAFFIC_ON_POLYLINE` computation
- Field mask allows requesting only needed data to reduce response size
- 3,000 queries per minute rate limit suitable for caching approach

**Implementation Details**:
- **Endpoint**: `POST https://routes.googleapis.com/directions/v2:computeRoutes`
- **Authentication**: `X-Goog-Api-Key` header (required)
- **Field Mask Required**: `X-Goog-FieldMask: routes.duration,routes.distanceMeters,routes.polyline.encodedPolyline,routes.travelAdvisory.speedReadingIntervals`
- **Request Structure**:
  ```json
  {
    "origin": {"location": {"latLng": {"latitude": 47.6062, "longitude": -122.3321}}},
    "destination": {"location": {"latLng": {"latitude": 45.5152, "longitude": -122.6784}}},
    "travelMode": "DRIVE",
    "routingPreference": "TRAFFIC_AWARE",
    "extraComputations": ["TRAFFIC_ON_POLYLINE"]
  }
  ```
- **Traffic Data**: Requires `extraComputations: ["TRAFFIC_ON_POLYLINE"]` and specific field mask
- **Rate Limits**: 3,000 QPM, exponential backoff for retries
- **Error Handling**: 400/403/429 status codes, proper field mask critical
- **Polyline Decoding**: Google polyline encoding, JSON escaped in responses

### Caltrans KML Feeds Integration
**Decision**: Use Caltrans KML data feeds for official road status, chain control, and incidents
**Rationale**:
- Provides authoritative California road data from official state sources
- Three separate feeds cover chain controls, lane closures, and CHP incidents
- KML format contains comprehensive geographic data with high precision
- Free public feeds with no API key requirements
- Updates: CHP incidents (1min), chain controls (1-5min), lane closures (5min)

**Implementation Details**:
- **Feed URLs**:
  - Chain Controls: `https://quickmap.dot.ca.gov/data/cc.kml` (10-50 entries)
  - Lane Closures: `https://quickmap.dot.ca.gov/data/lcs2way.kml` (20-100 entries)
  - CHP Incidents: `https://quickmap.dot.ca.gov/data/chp-only.kml` (50-200+ entries)
- **KML Structure**: Standard KML 2.2 with Placemark entries containing Point coordinates
- **Data Fields**: Name (identifier), Description (HTML with details), StyleUrl (closure type), Coordinates (WGS84 decimal degrees)
- **Coordinate Format**: `longitude,latitude,0` with 6+ decimal precision
- **Content Type**: HTML descriptions with CDATA blocks requiring parsing
- **Go Parsing**: Use `encoding/xml` with custom structs, handle CDATA and HTML extraction
- **Geographic Filtering**: Calculate distance from route coordinates to filter relevant incidents
- **Error Handling**: Network timeouts, malformed XML, empty feeds, encoding issues
- **Performance**: Files <1MB each, concurrent fetching recommended

### OpenWeatherMap API  
**Decision**: Use OpenWeatherMap API for weather data and alerts
**Rationale**:
- Comprehensive weather data including current conditions and alerts
- Good geographic coverage with lat/lon coordinate precision
- Free tier: 60 calls/minute, 1M calls/month, 1K alerts/day
- Well-documented REST API with reliable uptime

**Implementation Details**:
- **Current Weather**: `GET https://api.openweathermap.org/data/2.5/weather?lat={lat}&lon={lon}&appid={key}`
- **Weather Alerts**: `GET https://api.openweathermap.org/data/3.0/onecall?lat={lat}&lon={lon}&appid={key}&exclude=hourly,daily`
- **Authentication**: API key in query parameter (`appid={key}`)
- **Response Format**: JSON with structured weather data and alert objects
- **Coordinate Precision**: Up to 6 decimal places, WGS84 coordinate system
- **Alert Categories**: 14 types (extreme cold/hot, fog, wind, thunderstorms, etc.)
- **Alert Data**: Includes sender, event type, start/end times, descriptions, tags
- **Rate Limits**: 60/minute free tier, 429 status for rate limit exceeded
- **Error Handling**: 400 (bad params), 401 (bad API key), 404 (no data), 5xx (server)
- **Caching**: 10-minute intervals for current weather, 5-minute for alerts
- **Data Freshness**: Current weather real-time, alerts updated as issued

**Alternatives considered**:
- National Weather Service API: US-only coverage, less comprehensive  
- AccuWeather: More expensive, complex authentication

### Caching Strategy
**Decision**: In-memory caching with configurable refresh intervals
**Rationale**:
- Meets performance requirements (<5 second response times)
- Reduces external API calls and costs
- Simple to implement and debug
- Suitable for single-instance deployment

**Implementation approach**:
- Cache struct with mutex protection for concurrent access  
- Background go-routines for data refresh
- Configurable refresh intervals (default 5 minutes)
- Stale data detection (default 10 minutes = 2x refresh interval)

**Alternatives considered**:
- Redis: Unnecessary complexity for single instance
- File-based cache: Slower, persistence not required

### Configuration Management
**Decision**: Use Prefab's hierarchical configuration system
**Rationale**:
- YAML base configuration with environment variable overrides
- Supports route lists, location lists, API keys, refresh intervals
- Clean separation of config from code
- Easy deployment across different environments

**Configuration structure**:
```yaml
server:
  port: 8080
  cors_origins: ["*"]

routes:
  google_routes:
    refresh_interval: "5m"
    stale_threshold: "10m"
    api_key: "${GOOGLE_ROUTES_API_KEY}"
  caltrans_feeds:
    chain_controls:
      refresh_interval: "15m"  # Less frequent, changes slowly
      url: "https://quickmap.dot.ca.gov/data/cc.kml"
    lane_closures:
      refresh_interval: "10m"
      url: "https://quickmap.dot.ca.gov/data/lcs2way.kml"
    chp_incidents:
      refresh_interval: "5m"   # More frequent, incidents change quickly
      url: "https://quickmap.dot.ca.gov/data/chp-only.kml"
  monitored_routes:
    - name: "I-5 Seattle to Portland"
      id: "i5-sea-pdx"
      origin: {lat: 47.6062, lon: -122.3321}
      destination: {lat: 45.5152, lon: -122.6784}

weather:
  refresh_interval: "5m"
  stale_threshold: "10m" 
  openweather_api_key: "${OPENWEATHER_API_KEY}"
  locations:
    - {id: "seattle", name: "Seattle, WA", lat: 47.6062, lon: -122.3321}
    - {id: "portland", name: "Portland, OR", lat: 45.5152, lon: -122.6784}
```

### Testing Strategy
**Decision**: Multi-layered testing with CLI tools, contract tests, and integration tests
**Rationale**:
- CLI testing tools enable rapid development and debugging of external API clients
- Contract tests validate gRPC service definitions match proto specs
- Integration tests verify end-to-end API behavior
- Mock external APIs for reliable automated testing
- Real external API tests in separate integration suite

**Test structure**:
- `cmd/test-*/` - CLI tools for testing individual API clients
- `tests/contract/` - gRPC service contract tests
- `tests/integration/` - End-to-end API tests with mocked externals
- `tests/unit/` - Individual component tests
- `tests/external/` - Real external API integration tests (separate suite)

**CLI Testing Tools**:
- `test-google`: Test Google Routes API with coordinate pairs, validate JSON parsing
- `test-caltrans`: Test Caltrans KML feed parsing, geographic filtering, data extraction
- `test-weather`: Test OpenWeatherMap API calls, response parsing, location handling

**CLI Tool Features**:
- `--config` flag to load configuration file
- `--format json|yaml|table` for different output formats
- `--route-id` or `--location-id` for testing specific items
- `--verbose` flag for detailed debug output
- `--dry-run` flag for configuration validation without API calls

**Build and Development Tooling**:
- `Makefile` for standardized build, test, and deployment tasks
- Common targets: `build`, `test`, `proto`, `clean`, `docker`, `deploy`
- CLI tool compilation and installation via Make targets
- Automated protobuf generation and dependency management

### Project Structure
**Decision**: Standard Go project layout with proto-first design and CLI testing tools
```
├── api/v1/                  # Proto definitions
│   ├── roads.proto         
│   └── weather.proto       
├── internal/               # Private application code
│   ├── services/           # gRPC service implementations
│   ├── clients/            # External API clients (Google, Caltrans, OpenWeather)
│   ├── parsers/            # KML and data format parsers
│   ├── cache/              # In-memory caching
│   └── config/             # Configuration structs
├── cmd/                    # CLI applications
│   ├── server/             # Main gRPC server
│   │   └── main.go
│   ├── test-google/        # Google Routes API testing CLI
│   │   └── main.go
│   ├── test-caltrans/      # Caltrans KML feeds testing CLI
│   │   └── main.go
│   └── test-weather/       # OpenWeatherMap API testing CLI
│       └── main.go
├── tests/                  # Test suites
├── config.yaml             # Base configuration
├── Makefile                # Build, test, and deployment tasks
└── go.mod
```

## Research Conclusions

All technical unknowns have been resolved with detailed implementation specs:
- ✅ Go 1.21+ with gRPC ecosystem provides solid foundation
- ✅ Prefab framework reduces boilerplate and provides needed features
- ✅ Google Routes API v2 researched: field masks required, 3K QPM limits, traffic data extraction
- ✅ Caltrans KML feeds analyzed: XML structure, update frequencies (1-5min), geographic precision
- ✅ OpenWeatherMap API detailed: current weather + alerts, 60/min rate limits, coordinate-based queries
- ✅ KML parsing approach with `encoding/xml`, HTML extraction, geographic filtering
- ✅ Rate limiting strategies per API (Google: 3K QPM, OpenWeather: 60/min, Caltrans: no limits)
- ✅ Error handling patterns for each API type (HTTP codes, retry logic, malformed data)
- ✅ In-memory caching strategy with API-specific refresh rates (1-10min)
- ✅ Configuration approach supports deployment flexibility
- ✅ Testing strategy with CLI tools for each external API

**Critical Implementation Details Identified**:
- Google Routes requires mandatory field masks or returns errors
- Caltrans KML feeds need HTML parsing from CDATA blocks
- OpenWeatherMap alerts require separate One Call API 3.0 endpoint
- Geographic filtering needed for all feeds to match route coordinates
- Polyline decoding required for Google Routes traffic visualization

Ready to proceed with concrete implementation guidance.