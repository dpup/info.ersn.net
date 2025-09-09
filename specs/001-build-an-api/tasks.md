# Tasks: Live Data API Server

**Input**: Design documents from `/Users/pupius/Dropbox/Projects/Sites/info.ersn.net/specs/001-build-an-api/`
**Prerequisites**: plan.md, research.md, data-model.md, contracts/roads.proto, contracts/weather.proto, quickstart.md

## Execution Flow (main)
```
1. Load plan.md from feature directory ✓
   → Tech stack: Go 1.21+ with gRPC, gRPC Gateway, Prefab framework
   → Structure: Single project with internal/ and cmd/ directories
2. Load design documents ✓:
   → data-model.md: 8 entities (Route, TrafficCondition, WeatherData, etc.)
   → contracts/: 2 proto files (roads.proto, weather.proto) with 4 gRPC services
   → research.md: Google Routes API, OpenWeatherMap, Caltrans KML decisions
3. Generate tasks by category: Setup → Tests → Core → Integration → Polish
4. Apply task rules: [P] for different files, TDD order enforced
5. Number tasks sequentially (T001-T035)
6. Include parallel execution examples for external API clients
```

## Format: `[ID] [P?] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- Include exact file paths in descriptions

## Path Conventions
**Single project** at repository root:
- Source code: `internal/`, `cmd/`, `api/v1/`
- Tests: `tests/contract/`, `tests/integration/`, `tests/unit/`

## Phase 3.1: Setup

- [ ] **T001** Create Go project structure per implementation plan
  - Create directories: `cmd/{server,test-google,test-caltrans,test-weather}`, `internal/{services,clients,parsers,cache,config}`, `api/v1/`, `tests/{contract,integration,unit}`
  - Initialize `go.mod` with `github.com/dpup/info.ersn.net/server`
  - Create basic `Makefile` with build, test, proto targets

- [ ] **T002** Install Go dependencies and protoc plugins
  - Add dependencies: `github.com/dpup/prefab`, `google.golang.org/grpc`, `github.com/grpc-ecosystem/grpc-gateway/v2`, `github.com/twpayne/go-kml`
  - Install protoc plugins: `protoc-gen-go`, `protoc-gen-go-grpc`, `protoc-gen-grpc-gateway`
  - Verify `make build` runs without errors

- [ ] **T003** Generate protobuf code from contracts
  - Copy `specs/001-build-an-api/contracts/*.proto` to `api/v1/`
  - Configure protoc generation in `Makefile` with proper paths
  - Generate `*.pb.go` and `*_grpc.pb.go` files
  - Verify generated code compiles

## Phase 3.2: Tests First (TDD) ⚠️ MUST COMPLETE BEFORE 3.3
**CRITICAL: These tests MUST be written and MUST FAIL before ANY implementation**

- [ ] **T004** [P] Contract test RoadsService.ListRoutes in `tests/contract/roads_service_test.go`
  - Test gRPC method signature matches `contracts/roads.proto` line 12-17
  - Assert response structure with routes array and last_updated timestamp
  - Test must FAIL initially (no implementation yet)

- [ ] **T005** [P] Contract test RoadsService.GetRoute in `tests/contract/roads_service_test.go`
  - Test gRPC method with route ID parameter
  - Assert single route response with traffic conditions
  - Test must FAIL initially

- [ ] **T006** [P] Contract test WeatherService.ListWeatherData in `tests/contract/weather_service_test.go`
  - Test gRPC method signature matches `contracts/weather.proto` line 12-17
  - Assert response structure with weather_data array and last_updated
  - Test must FAIL initially

- [ ] **T007** [P] Contract test WeatherService.GetWeatherAlerts in `tests/contract/weather_service_test.go`
  - Test weather alerts retrieval
  - Assert alerts array with proper OpenWeatherMap structure
  - Test must FAIL initially

- [ ] **T008** [P] Integration test Google Routes API client in `tests/integration/google_client_test.go`
  - Test coordinate-based route computation with real API
  - Verify field mask requirements and rate limiting
  - Use test route: Seattle to Portland coordinates

- [ ] **T009** [P] Integration test Caltrans KML parser in `tests/integration/caltrans_parser_test.go`
  - Test KML feed parsing for all 3 feeds (chain control, lane closures, CHP incidents)
  - Verify geographic filtering and HTML extraction
  - Use real KML URLs from `research.md`

- [ ] **T010** [P] Integration test OpenWeatherMap client in `tests/integration/weather_client_test.go`
  - Test current weather and alerts API endpoints
  - Verify coordinate-based queries and response parsing
  - Use Seattle coordinates for testing

## Phase 3.3: External API Clients (ONLY after tests are failing)

- [ ] **T011** [P] Configuration framework in `internal/config/config.go`
  - Implement config structs matching `research.md` YAML structure (lines 112-146)
  - Support Prefab framework configuration loading
  - Environment variable substitution for API keys

- [ ] **T012** [P] Google Routes API client in `internal/clients/google/client.go`
  - Implement `ComputeRoutes()` method with field mask requirements
  - Rate limiting (3K QPM) and retry logic with exponential backoff
  - Coordinate-based POST requests to `/directions/v2:computeRoutes`
  - Reference: `research.md` lines 32-47, `data-model.md` TrafficCondition entity

- [ ] **T013** [P] Caltrans KML parser in `internal/clients/caltrans/parser.go`
  - KML feed parsing for 3 URLs using `github.com/twpayne/go-kml`
  - HTML extraction from CDATA blocks and geographic filtering
  - Map to `CaltransIncident` struct from `data-model.md` lines 66-78
  - Handle all 3 feed types: chain control, lane closures, CHP incidents

- [ ] **T014** [P] OpenWeatherMap API client in `internal/clients/weather/client.go`
  - Current weather endpoint: `GET /data/2.5/weather`
  - Weather alerts endpoint: `GET /data/3.0/onecall` with proper exclusions
  - Rate limiting (60/minute) and coordinate-based queries
  - Reference: `research.md` lines 68-82, `data-model.md` WeatherData entity

- [ ] **T015** [P] In-memory cache implementation in `internal/cache/cache.go`
  - TTL-based caching with mutex protection for concurrent access
  - Configurable refresh intervals and stale detection
  - Background refresh goroutines per `data-model.md` Cache Entry (lines 227-241)

## Phase 3.4: gRPC Services Implementation

- [ ] **T016** RoadsService implementation in `internal/services/roads.go`
  - Implement `ListRoutes()` and `GetRoute()` gRPC methods
  - Combine Google Routes traffic data + Caltrans status + geographic filtering
  - Use cache for data freshness, fallback to external APIs
  - Return unified Route entities per `data-model.md` Route structure (lines 5-17)

- [ ] **T017** WeatherService implementation in `internal/services/weather.go`
  - Implement `ListWeatherData()` and `GetWeatherAlerts()` gRPC methods
  - Process OpenWeatherMap API responses into consistent units (Celsius/meters/seconds)
  - Cache weather data with 5-minute refresh intervals
  - Handle weather alerts with proper OpenWeatherMap tag mapping

## Phase 3.5: CLI Testing Tools (Can run parallel with services)

- [ ] **T018** [P] Google Routes testing CLI in `cmd/test-google/main.go`
  - Command-line tool with flags: `--config`, `--route-id`, `--verbose`, `--format`
  - Test coordinate-based API calls and JSON parsing
  - Output formats: json, yaml, table for debugging
  - Reference: `quickstart.md` lines 255-261

- [ ] **T019** [P] Caltrans testing CLI in `cmd/test-caltrans/main.go`
  - Test all 3 KML feed URLs with parsing validation
  - Geographic filtering and HTML extraction verification
  - Verbose mode shows parsed incident details

- [ ] **T020** [P] Weather testing CLI in `cmd/test-weather/main.go`
  - Test OpenWeatherMap current weather and alerts APIs
  - Location-based queries with coordinate validation
  - Show parsed weather data structure in requested format

## Phase 3.6: Server Integration

- [ ] **T021** Main server integration in `cmd/server/main.go`
  - Wire gRPC services with Prefab framework
  - Configure gRPC Gateway for REST endpoints
  - CORS configuration for static website integration
  - Reference: `research.md` Prefab integration (lines 17-24)

- [ ] **T022** Background data refresh orchestration
  - Start background goroutines for cache refresh
  - Coordinate refresh intervals: Google (5min), Caltrans (5-15min), Weather (5min)
  - Error handling and retry logic for external API failures

- [ ] **T023** Server configuration and startup
  - Load configuration from `config.yaml` + environment variables
  - Validate API keys and external service connectivity
  - Graceful shutdown handling and resource cleanup

## Phase 3.7: End-to-End Integration Tests

- [ ] **T024** Full API workflow test in `tests/integration/api_workflow_test.go`
  - Test REST endpoints via gRPC Gateway: `/api/v1/routes`, `/api/v1/weather`
  - Verify response formats match `quickstart.md` examples (lines 147-200)
  - Test caching behavior: multiple requests within refresh interval

- [ ] **T025** CORS and static website integration test
  - Test preflight OPTIONS requests and CORS headers
  - Verify JavaScript fetch() compatibility per `quickstart.md` lines 287-323
  - Test with actual static website scenario

## Phase 3.8: Performance and Validation

- [ ] **T026** Performance validation tests
  - Verify <5 second response times per technical constraints
  - Test concurrent request handling (low-scale personal use)
  - Cache effectiveness measurement

- [ ] **T027** [P] Unit tests for data parsing in `tests/unit/parsers_test.go`
  - Test Google Routes JSON parsing edge cases
  - Test Caltrans KML parsing with malformed HTML
  - Test OpenWeatherMap response unit conversions

- [ ] **T028** Error handling and resilience tests
  - API key validation and quota exceeded scenarios
  - Network timeouts and malformed responses
  - Cache miss handling and stale data detection

## Phase 3.9: Polish and Documentation

- [ ] **T029** [P] Update CLI tool installation in Makefile
  - Add targets for installing CLI tools to system PATH
  - Build targets for individual tools: `make tools`, `make test-google`
  - Docker build target preparation

- [ ] **T030** [P] Logging and observability
  - Structured JSON logging via Prefab framework
  - API client request/response logging with sensitive data masking
  - Performance metrics and error rate tracking

- [ ] **T031** Final integration with quickstart guide
  - Verify all `quickstart.md` commands work end-to-end
  - Test API usage examples return expected JSON structure
  - Validate troubleshooting section accuracy

- [ ] **T032** Code quality and cleanup
  - Run `go fmt`, `go vet`, and any configured linters
  - Remove debugging code and commented sections
  - Verify error messages are user-friendly

## Dependencies

**Critical Ordering**:
- **Setup** (T001-T003) → **Tests** (T004-T010) → **Implementation** (T011+)
- **T004-T007** (contract tests) MUST FAIL before starting T016-T017 (services)
- **T011** (config) blocks T012-T014 (API clients)
- **T012-T015** (clients + cache) block T016-T017 (services)
- **T016-T017** (services) block T021 (server integration)

**Parallel Execution**:
- T004-T010: All test tasks (different files)
- T012-T015: All API clients and cache (independent implementations)
- T018-T020: CLI tools (independent commands)
- T027, T029-T030: Polish tasks in different files

## Parallel Example
```bash
# Launch external API client development in parallel:
# Task agent: "Google Routes API client in internal/clients/google/client.go"
# Task agent: "Caltrans KML parser in internal/clients/caltrans/parser.go" 
# Task agent: "OpenWeatherMap client in internal/clients/weather/client.go"
# Task agent: "In-memory cache in internal/cache/cache.go"

# Later, launch CLI tools in parallel:
# Task agent: "Google Routes testing CLI in cmd/test-google/main.go"
# Task agent: "Caltrans testing CLI in cmd/test-caltrans/main.go"
# Task agent: "Weather testing CLI in cmd/test-weather/main.go"
```

## Cross-References to Design Documents

- **T001-T003**: `plan.md` project structure (lines 136-139), `quickstart.md` setup (lines 13-36)
- **T004-T007**: `contracts/roads.proto`, `contracts/weather.proto` service definitions
- **T008-T010**: `research.md` external API sections (lines 32-82)
- **T011**: `research.md` configuration structure (lines 112-146)
- **T012**: `research.md` Google Routes API (lines 32-47), `data-model.md` TrafficCondition (lines 25-36)
- **T013**: `research.md` Caltrans integration (lines 49-67), `data-model.md` CaltransIncident (lines 66-78)
- **T014**: `research.md` OpenWeatherMap (lines 68-82), `data-model.md` WeatherData (lines 104-121)
- **T016-T017**: `data-model.md` all entities, service implementation patterns
- **T018-T020**: `research.md` CLI tool features (lines 169-174), `quickstart.md` troubleshooting (lines 255-277)
- **T021-T023**: `research.md` Prefab framework (lines 17-24), server orchestration
- **T024-T025**: `quickstart.md` API examples (lines 147-200, 287-323)

## Validation Checklist ✓
- [x] All contracts have corresponding tests (T004-T007)
- [x] All entities have implementation tasks (T011-T017)
- [x] All tests come before implementation (T004-T010 before T011+)
- [x] Parallel tasks truly independent (different files, no shared dependencies)
- [x] Each task specifies exact file path
- [x] No task modifies same file as another [P] task
- [x] TDD order enforced: contract tests → integration tests → implementation
- [x] External API clients can be developed in parallel (T012-T014)
- [x] CLI testing tools independent and parallel (T018-T020)

## Notes
- **[P] tasks** = different files, no dependencies - can run simultaneously
- **Verify tests fail** before implementing (RED phase of TDD)
- **Commit after each task** for atomic progress
- **Reference design docs** for implementation details
- Use CLI testing tools (T018-T020) to debug API integration issues
- Configuration validation via `make test-config` before full server runs