# Tasks: Smart Route-Relevant Alert Filtering

**Input**: Design documents from `/specs/002-improvements-to-handling/`
**Prerequisites**: plan.md (required), research.md, data-model.md, contracts/

## Path Conventions
- **Single project**: `internal/`, `cmd/`, `api/` at repository root (per plan.md)
- Go 1.21+ with gRPC, gRPC Gateway, Prefab framework
- Libraries in `internal/lib/`, CLI tools in `cmd/`

## Phase 3.1: Setup

- [ ] T001 Initialize Go module dependencies for geo-spatial libraries and OpenAI
- [ ] T002 [P] Configure linting and formatting tools (golangci-lint, gofmt)
- [ ] T003 Create project structure for three libraries in internal/lib/

## Phase 3.2: Tests First (TDD) ⚠️ MUST COMPLETE BEFORE 3.3
**CRITICAL: These tests MUST be written and MUST FAIL before ANY implementation**

### Library Contract Tests
- [ ] T004 [P] Contract test geo-utils interface in internal/lib/geo/geo_test.go
- [ ] T005 [P] Contract test alert-enhancer interface in internal/lib/alerts/enhancer_test.go
- [ ] T006 [P] Contract test route-matcher interface in internal/lib/routing/matcher_test.go

### gRPC Service Contract Tests
- [ ] T007 [P] Contract test updated RoadAlert message fields in api/v1/roads_test.go
- [ ] T008 [P] Contract test GetProcessingMetrics RPC method in api/v1/roads_test.go

### Integration Tests (User Stories)
- [ ] T009 [P] Integration test route-relevant alert filtering in tests/integration/alert_filtering_test.go
- [ ] T010 [P] Integration test AI description enhancement in tests/integration/ai_enhancement_test.go
- [ ] T011 [P] Integration test polyline-based classification in tests/integration/polyline_classification_test.go

## Phase 3.3: Core Implementation (ONLY after tests are failing)

### Library Implementations
- [ ] T012 [P] Geo-utils library implementation in internal/lib/geo/geo.go
- [ ] T013 [P] Alert-enhancer library implementation in internal/lib/alerts/enhancer.go
- [ ] T014 Route-matcher library implementation in internal/lib/routing/matcher.go (depends on geo-utils)

### CLI Tools
- [ ] T015 [P] CLI tool test-geo-utils in cmd/test-geo-utils/main.go
- [ ] T016 [P] CLI tool test-alert-enhancer in cmd/test-alert-enhancer/main.go
- [ ] T017 [P] CLI tool test-route-matcher in cmd/test-route-matcher/main.go

## Phase 3.4: Integration

- [ ] T018 Update gRPC Protocol Buffer definitions in api/v1/roads.proto
- [ ] T019 Enhance Caltrans parser with route classification in internal/clients/caltrans/parser.go
- [ ] T020 Enhance roads service with alert processing pipeline in internal/services/roads.go
- [ ] T021 Add OpenAI API configuration to Prefab framework
- [ ] T022 Connect libraries to existing service layer

## Phase 3.5: Polish

- [ ] T023 [P] Unit tests for geographic calculations in tests/unit/geo_test.go
- [ ] T024 [P] Unit tests for AI enhancement logic in tests/unit/ai_test.go
- [ ] T025 [P] Unit tests for route classification in tests/unit/routing_test.go
- [ ] T026 Performance tests for <2s API response time
- [ ] T027 [P] Update CLAUDE.md with new libraries and features
- [ ] T028 Execute quickstart.md validation scenarios

## Dependencies
- Tests (T004-T011) before implementation (T012-T022)
- T012 (geo-utils) blocks T014 (route-matcher)
- T018 (protobuf) before T019-T020 (service integration)
- Implementation before polish (T023-T028)

## Parallel Example
```
# Launch T004-T006 together (library contract tests):
Task: "Contract test geo-utils interface in internal/lib/geo/geo_test.go"
Task: "Contract test alert-enhancer interface in internal/lib/alerts/enhancer_test.go" 
Task: "Contract test route-matcher interface in internal/lib/routing/matcher_test.go"

# Launch T012-T013 together (independent library implementations):
Task: "Geo-utils library implementation in internal/lib/geo/geo.go"
Task: "Alert-enhancer library implementation in internal/lib/alerts/enhancer.go"
```

## Notes
- [P] tasks = different files, no dependencies
- Verify tests fail before implementing
- Commit after each task
- Three core libraries: geo-utils, alert-enhancer, route-matcher
- CLI tools provide debugging capabilities for production
- TDD mandatory: RED-GREEN-Refactor cycle enforced

## Validation Checklist
*GATE: Checked before task execution*

- [ ] All library interfaces have corresponding contract tests
- [ ] All gRPC message updates have tests
- [ ] All integration tests come before implementation
- [ ] Parallel tasks truly independent (different files)
- [ ] Each task specifies exact file path
- [ ] No task modifies same file as another [P] task

