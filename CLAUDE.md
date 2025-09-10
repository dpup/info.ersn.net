# ERSN Info Server Development Guidelines

Auto-generated from feature spec 001-build-an-api. Last updated: 2025-09-09

## Active Technologies

**Language/Version**: Go 1.21+  
**Primary Dependencies**: gRPC, gRPC Gateway, Prefab framework (github.com/dpup/prefab), Protocol Buffers  
**Storage**: In-memory caching (no persistent storage required)  
**Testing**: Go testing framework with testify, contract tests for gRPC services  
**Target Platform**: Linux/macOS server, containerizable

## Project Structure
```
/
├── api/v1/                     # Protocol Buffer definitions
│   ├── roads.proto            # gRPC service for road conditions  
│   ├── weather.proto          # gRPC service for weather data
│   └── common.proto           # Shared proto definitions
├── cmd/                       # CLI applications
│   ├── server/                # Main API server
│   ├── test-google/           # Google Routes API testing tool
│   ├── test-caltrans/         # Caltrans data testing tool
│   └── test-weather/          # Weather API testing tool
├── internal/                  # Private application code
│   ├── services/              # gRPC service implementations
│   ├── clients/               # External API clients
│   │   ├── google/           # Google Routes API client
│   │   ├── caltrans/         # Caltrans KML parser
│   │   └── weather/          # OpenWeatherMap client
│   ├── cache/                # In-memory caching with TTL
│   └── config/               # Configuration management
├── tests/                    # Test files
│   ├── contract/             # gRPC contract tests
│   ├── integration/          # External API integration tests
│   └── unit/                 # Unit tests
├── specs/001-build-an-api/   # Feature specification and design docs
└── Makefile                  # Build automation
```

## Commands

### Build & Development
```bash
# Generate protobuf code
make proto

# Build all binaries
make build

# Build specific components
make server
make tools

# Run server in foreground
make run

# Run server in background for testing
make run-bg

# Stop background server
make stop

# Clean build artifacts
make clean
```

### Testing
```bash
# Run all tests
make test

# Run specific test suites
make test-contract
make test-integration
make test-unit

# Test external API clients
./bin/test-google
./bin/test-caltrans  
./bin/test-weather
```

### API Testing
```bash
# Test live endpoints
curl http://localhost:8080/api/v1/routes
curl http://localhost:8080/api/v1/weather

# Format JSON responses
curl -s http://localhost:8080/api/v1/routes | jq .
```

## Code Style

**Go Conventions**:
- Follow standard Go formatting: `go fmt`, `go vet`
- Use structured logging via Prefab framework
- gRPC-first design with Protocol Buffers
- Environment variables for sensitive configuration
- Graceful error handling with proper context

**API Design**:
- REST endpoints via gRPC Gateway
- CORS enabled for static website integration
- Consistent JSON response format
- No authentication required
- Cache-friendly with appropriate TTLs

## Development Workflow

This project follows the **Specification-First Development** workflow:

### 1. Feature Specification (`/specify`)
**Command**: Create `specs/[###-feature]/spec.md`
- Focus on WHAT users need and WHY (not HOW to implement)
- Written for business stakeholders, not developers
- Must include User Scenarios, Requirements, and Key Entities
- No technical implementation details

### 2. Implementation Planning (`/plan`) 
**Command**: Creates design documents in `specs/[###-feature]/`
- `plan.md` - Technical approach and architecture decisions
- `research.md` - Technology research and external API analysis  
- `data-model.md` - Entities, relationships, validation rules
- `contracts/` - API contracts (Protocol Buffers)
- `quickstart.md` - Setup instructions and usage examples

### 3. Task Generation (`/tasks`)
**Command**: Creates `specs/[###-feature]/tasks.md`
- Numbered, ordered implementation tasks (T001-T035)
- Test-Driven Development enforced: Tests → Implementation
- Parallel execution markers [P] for independent tasks
- Cross-references to design documents

### 4. Implementation Execution
**Approach**: Follow constitutional principles
- **TDD Mandatory**: Write failing tests before implementation
- **Library-First**: Every feature as standalone, testable library
- **CLI Interface**: Each library exposes functionality via CLI tools
- **Integration Testing**: Focus on external API contracts

## Environment Setup

**Required Environment Variables**:
```bash
# API Keys (required for production)
export GOOGLE_API_KEY="your-google-routes-api-key"
export OPENWEATHER_API_KEY="your-openweather-api-key"

# Optional Configuration
export PORT=8080
```

**Configuration Files**:
- `config.yaml` - Application configuration (API refresh intervals, route definitions)
- Environment variables override config file values for secrets

## External API Integration

**Google Routes API**:
- Rate limit: 3,000 QPM (queries per minute)
- Requires field mask for optimal performance
- Coordinate-based POST requests to `/directions/v2:computeRoutes`

**OpenWeatherMap API**:
- Rate limit: 60 calls/minute (free tier)
- Current weather: `/data/2.5/weather`
- Weather alerts: `/data/3.0/onecall`

**Caltrans KML Feeds**:
- Chain control status, lane closures, CHP incidents
- XML parsing with geographic filtering
- Refresh intervals: 5-15 minutes based on data type

## API Endpoints

**Routes Service** (`/api/v1/routes`):
- `GET /api/v1/routes` - List all configured routes with current conditions
- `GET /api/v1/routes/{route_id}` - Get specific route details
- Returns: Route status, traffic conditions, chain controls, alerts

**Weather Service** (`/api/v1/weather`):
- `GET /api/v1/weather` - Current weather for all configured locations  
- `GET /api/v1/weather/alerts` - Active weather alerts
- Returns: Temperature, conditions, visibility, wind, alerts

## Performance & Monitoring

**Response Time Targets**:
- Weather API: < 1 second
- Routes API: < 2 seconds  
- Cache refresh: 5-minute intervals
- Stale data threshold: 10 minutes

**Logging**:
- Structured JSON logs via Prefab framework
- Request/response logging with sensitive data masking
- External API call tracking with rate limit monitoring

## Recent Changes

**Feature 001-build-an-api** (2025-09-09):
- Complete ERSN Info Server with Google Routes and OpenWeatherMap integration
- gRPC services with HTTP gateway for REST API access
- In-memory caching with configurable refresh intervals
- CLI testing tools for each external API integration
- Production-ready with CORS support for static website integration

<!-- MANUAL ADDITIONS START -->

## Development Tips

**Testing External APIs**:
- Use CLI tools (`test-google`, `test-caltrans`, `test-weather`) for debugging
- Check API key restrictions in Google Cloud Console (no HTTP referrer blocks)
- Monitor rate limits and implement proper backoff strategies

**Debugging Common Issues**:
- **Google Routes API 403**: Check API key referrer restrictions
- **Server won't start**: Verify environment variables are set
- **Slow responses**: Check external API timeouts and cache hit rates
- **Stale data**: Verify background refresh goroutines are running

**Adding New Routes**:
1. Update `config.yaml` with new route coordinates
2. Test with `./bin/test-google` using new coordinates  
3. Restart server to pick up configuration changes
4. Verify new route appears in `/api/v1/routes` response

**Adding New Weather Locations**:
1. Update `config.yaml` weather locations section
2. Test with `./bin/test-weather` using new coordinates
3. Restart server and verify in `/api/v1/weather` response

<!-- MANUAL ADDITIONS END -->