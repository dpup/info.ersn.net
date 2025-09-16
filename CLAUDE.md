# ERSN Info Server Development Guidelines

Last updated: 2025-09-13

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
├── bin/                        # Compiled binaries
├── cmd/                       # CLI applications
│   ├── server/                # Main API server
│   ├── test-google/           # Google Routes API testing tool
│   ├── test-caltrans/         # Caltrans data testing tool
│   └── test-weather/          # Weather API testing tool
├── internal/                  # Private application code
│   ├── services/              # gRPC service implementations
│   ├── clients/               # External API clients
│   ├── cache/                 # In-memory caching with TTL
│   ├── config/                # Configuration management
│   └── lib/                   # Shared libraries
├── tests/                     # Test files and test data
└── Makefile                   # Build automation
```

## Commands

Whenever possible you MUST use a command provided by the makefile. If you need additional functionality
discuss with the operator improvements to the makefile commands.

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

**IMPORTANT**: Always use `make run-bg` to start the server in background, not manual `./bin/server &` commands. The Makefile handles proper process management.

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
curl http://localhost:8080/api/v1/roads
curl http://localhost:8080/api/v1/weather

# Format JSON responses
curl -s http://localhost:8080/api/v1/roads | jq .
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

For new features, follow this structured approach:

1. **Plan**: Understand requirements and design approach
2. **Implement**: Write tests first, then implementation
3. **Test**: Validate with unit tests and integration tests
4. **Document**: Update relevant documentation

**Development Principles**:
- **Test-Driven Development**: Write failing tests before implementation
- **Library-First**: Build standalone, testable libraries
- **CLI Testing Tools**: Each external API gets a dedicated test tool
- **Integration Focus**: Validate external API contracts

## Environment Setup

**Required Environment Variables**:
```bash
# API Keys (required for production)
export PF__GOOGLE_ROUTES__API_KEY="your-google-routes-api-key"
export PF__OPENWEATHER__API_KEY="your-openweather-api-key"
export PF__OPENAI__API_KEY="your-openai-api-key"  # For AI-enhanced alerts

# Optional Configuration
export PORT=8080
```

**Configuration Files**:
- `prefab.yaml` - Application configuration (API refresh intervals, route definitions)
- Environment variables override config file values for secrets
- Use `.envrc` for local development (already in .gitignore)

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

**OpenAI API** (Optional):
- **AI-Enhanced Road Status Determination**: Intelligently analyzes traffic incidents to determine accurate road status (open/restricted/closed)
- **Status Explanations**: Provides clear explanations when roads are restricted or closed (populates `status_explanation` field)
- **Smart Classification**: Distinguishes between mainline road closures vs ramp/exit closures for accurate status determination
- **Alert Enhancement**: Processes raw Caltrans data into user-friendly alert descriptions
- **Structured Outputs**: Uses OpenAI structured outputs for consistent response format
- **Content-Based Caching**: 24-hour cache prevents duplicate AI calls for identical content

## API Endpoints

**Roads Service** (`/api/v1/roads`):
- `GET /api/v1/roads` - List all configured roads with current conditions
- `GET /api/v1/roads/{road_id}` - Get specific road details
- `GET /api/v1/roads/metrics` - Get alert processing metrics
- Returns: Road status, status explanations, traffic conditions, chain controls, AI-enhanced alerts

**Key API Response Fields**:
- `status`: Current road status (OPEN/RESTRICTED/CLOSED/MAINTENANCE)
- `status_explanation`: AI-generated explanation when status is RESTRICTED or CLOSED
- `alerts[].description`: AI-enhanced human-readable alert descriptions
- `alerts[].condensed_summary`: Mobile-optimized short summaries
- `alerts[].impact`: AI-assessed impact levels (none/light/moderate/severe)
- `alerts[].metadata`: Structured additional information from AI analysis

**Weather Service** (`/api/v1/weather`):
- `GET /api/v1/weather` - Current weather for all configured locations
- `GET /api/v1/weather/{location_id}` - Get specific location weather
- `GET /api/v1/weather/alerts` - Active weather alerts
- Returns: Temperature, conditions, visibility, wind, alerts

## Performance & Monitoring

**Response Time Targets**:
- Weather API: < 1 second
- Roads API: < 2 seconds  
- Cache refresh: 5-minute intervals
- Stale data threshold: 10 minutes

**Logging**:
- Structured JSON logs via Prefab framework
- Request/response logging with sensitive data masking
- External API call tracking with rate limit monitoring

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

**Adding New Roads**:
1. Update `prefab.yaml` with new road coordinates
2. Test with `./bin/test-google` using new coordinates
3. Restart server to pick up configuration changes
4. Verify new road appears in `/api/v1/roads` response

**Adding New Weather Locations**:
1. Update `prefab.yaml` weather locations section
2. Test with `./bin/test-weather` using new coordinates
3. Restart server and verify in `/api/v1/weather` response

## AI Enhancement System

**Road Status Determination**:
- AI analyzes Caltrans incident data to determine accurate road status
- Distinguishes between mainline closures (status: CLOSED) vs ramp closures (status: RESTRICTED)
- Provides clear explanations in `status_explanation` field when roads are not fully open
- Examples: "Right lane blocked due to accident" vs "Off-ramp closure to Treasure Island"

**Alert Processing Pipeline**:
1. **Content Hashing**: Generate hash of raw alert content for caching
2. **Cache Check**: Check 24-hour cache to avoid duplicate OpenAI calls
3. **AI Analysis**: If cache miss, send to OpenAI for enhancement and status determination
4. **Response Processing**: Parse structured OpenAI response into API-ready format
5. **Cache Storage**: Store enhanced result with 24-hour TTL

**AI Enhancement Features**:
- **Human-Readable Descriptions**: Converts technical Caltrans language to clear, actionable information
- **Impact Assessment**: Categorizes impact as none/light/moderate/severe
- **Duration Estimates**: Provides duration context (unknown/< 1 hour/several hours/ongoing)
- **Condensed Summaries**: Creates mobile-friendly short descriptions
- **Structured Metadata**: Extracts additional context (lanes affected, emergency services, etc.)

**Development Best Practices**:
- Monitor OpenAI API usage and costs through logging
- Test AI enhancements with `./bin/test-caltrans` tool
- Verify status determination logic with different incident types
- Check cache hit rates to ensure efficient AI usage
- Validate structured output parsing for robustness

**Security Guidelines**:
- API keys are stored in `.envrc` (git-ignored)
- Never commit real API keys to the repository
- Use placeholder examples in documentation and configs
- Rotate API keys if they're accidentally exposed

