# Tasks: Background Processing for Incident Content Caching

**Input**: Design documents from `/specs/003-the-system-now/`
**Prerequisites**: plan.md (required), research.md, data-model.md, contracts/

## Execution Flow (main)
```
1. Load plan.md from feature directory
   → If not found: ERROR "No implementation plan found"
   → Extract: tech stack, libraries, structure
2. Load optional design documents:
   → data-model.md: Extract entities → model tasks
   → contracts/: Each file → contract test task
   → research.md: Extract decisions → setup tasks
3. Generate tasks by category:
   → Setup: project init, dependencies, linting
   → Tests: contract tests, integration tests
   → Core: models, services, CLI commands
   → Integration: DB, middleware, logging
   → Polish: unit tests, performance, docs
4. Apply task rules:
   → Different files = mark [P] for parallel
   → Same file = sequential (no [P])
   → Tests before implementation (TDD)
5. Number tasks sequentially (T001, T002...)
6. Generate dependency graph
7. Create parallel execution examples
8. Validate task completeness:
   → All contracts have tests?
   → All entities have models?
   → All endpoints implemented?
9. Return: SUCCESS (tasks ready for execution)
```

## Format: `[ID] [P?] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- Include exact file paths in descriptions

## Phase 3.1: Setup
- [ ] T001 Extend existing cache package with content-based methods in `internal/cache/cache.go`
- [ ] T002 Add Store configuration section to existing `internal/config/config.go` 
- [ ] T003 [P] Configure Makefile targets for incident processing tests

## Phase 3.2: Tests First (TDD) ⚠️ MUST COMPLETE BEFORE 3.3
**CRITICAL: These tests MUST be written and MUST FAIL before ANY implementation**
- [ ] T004 [P] Contract test IncidentContentHasher interface in `tests/contract/test_incident_hasher.go`
- [ ] T005 [P] Contract test ProcessedIncidentStore interface in `tests/contract/test_processed_store.go`
- [ ] T006 [P] Contract test BackgroundIncidentProcessor interface in `tests/contract/test_background_processor.go`
- [ ] T007 [P] Contract test AsyncAlertEnhancer interface in `tests/contract/test_async_enhancer.go`
- [ ] T008 [P] Integration test content hash deduplication in `tests/integration/test_content_deduplication.go`
- [ ] T009 [P] Integration test background processing pipeline in `tests/integration/test_background_pipeline.go`
- [ ] T010 [P] Integration test <200ms API response time in `tests/integration/test_response_performance.go`

## Phase 3.3: Core Implementation (ONLY after tests are failing)
- [ ] T011 [P] IncidentContentHash entity model in `internal/lib/incident/types.go`
- [ ] T012 [P] ProcessedIncidentCache entity model in `internal/lib/incident/types.go`
- [ ] T013 [P] IncidentContentHasher implementation in `internal/lib/incident/hasher.go`
- [ ] T014 [P] ProcessedIncidentStore implementation in `internal/cache/incident_store.go`
- [ ] T015 [P] BackgroundIncidentProcessor implementation in `internal/lib/incident/processor.go`
- [ ] T016 AsyncAlertEnhancer wrapper implementation in `internal/lib/incident/async_enhancer.go`
- [ ] T017 StoreConfig integration in `internal/config/config.go`
- [ ] T018 Content hash generation utilities in `internal/lib/incident/utils.go`

## Phase 3.4: Service Integration
- [ ] T019 Integrate AsyncAlertEnhancer with RoadsService in `internal/services/roads.go`
- [ ] T020 Add background processing startup to server main in `cmd/server/main.go`
- [ ] T021 Extend test-caltrans CLI with content hash testing in `cmd/test-caltrans/main.go`
- [ ] T022 Add Store config validation in `internal/config/config.go`
- [ ] T023 Implement cache metrics and monitoring in `internal/cache/metrics.go`

## Phase 3.5: Performance Optimization
- [ ] T024 Add background processor lifecycle management in `internal/services/roads.go`
- [ ] T025 Implement cache warming for common incidents in `internal/lib/incident/warmer.go`
- [ ] T026 Add OpenAI rate limiting and circuit breaker in `internal/lib/incident/rate_limiter.go`
- [ ] T027 Optimize content hash normalization performance in `internal/lib/incident/hasher.go`

## Phase 3.6: Polish & Validation
- [ ] T028 [P] Unit tests for content hash generation in `tests/unit/test_hasher_unit.go`
- [ ] T029 [P] Unit tests for cache store operations in `tests/unit/test_store_unit.go`
- [ ] T030 [P] Performance benchmarks for cache hit rates in `tests/performance/test_cache_performance.go`
- [ ] T031 [P] Memory usage profiling for content cache in `tests/performance/test_memory_profile.go`
- [ ] T032 Add cache metrics to existing logging in `internal/services/roads.go`
- [ ] T033 Document configuration options in existing `CLAUDE.md`
- [ ] T034 Validate 80-90% OpenAI cost reduction in `tests/integration/test_cost_savings.go`
- [ ] T035 Run performance tests and validate <200ms API responses

## Dependencies

**Phase Order**: 3.1 → 3.2 → 3.3 → 3.4 → 3.5 → 3.6

**Critical TDD Requirement**: All contract tests (T004-T010) MUST be written and MUST FAIL before implementing any core functionality (T011-T018).

**Key Dependencies**:
- T002 (Store config) before T017 (StoreConfig integration)
- T013 (IncidentContentHasher) before T019 (RoadsService integration)
- T014 (ProcessedIncidentStore) before T019 (RoadsService integration)
- T015 (BackgroundIncidentProcessor) before T020 (server startup integration)
- T016 (AsyncAlertEnhancer) before T019 (RoadsService integration)

## Parallel Execution Examples

**Setup Phase (can run together)**:
```bash
# Run these tasks in parallel - different files
Task --subagent code-architect T001 T002 T003
```

**Contract Tests (MUST run in parallel for efficiency)**:
```bash
# All contract tests can run in parallel - different files
Task --subagent test-runner-linter T004 T005 T006 T007 T008 T009 T010
```

**Core Models (can run in parallel)**:
```bash
# Entity implementations - different files  
Task --subagent code-architect T011 T012 T013 T014 T015 T016
```

**Performance Tests (final validation)**:
```bash
# Performance validation - different test files
Task --subagent test-runner-linter T028 T029 T030 T031 T034
```

## Success Criteria

1. **All contract tests fail initially** (validates TDD process)
2. **All contract tests pass after implementation** (validates interface compliance)
3. **API responses < 200ms** (validates performance requirement)
4. **80-90% reduction in OpenAI API calls** (validates cost efficiency)
5. **Cache expires 1 hour after incident disappears** (validates TTL requirement)
6. **Background processing handles incident batches** (validates async processing)
7. **Content-based deduplication works across feed refreshes** (validates core functionality)

## File Path Summary

**New Files Created**:
- `internal/lib/incident/types.go` - Core entities
- `internal/lib/incident/hasher.go` - Content hash generation
- `internal/lib/incident/processor.go` - Background processing
- `internal/lib/incident/async_enhancer.go` - Async alert enhancement wrapper
- `internal/lib/incident/utils.go` - Hash utilities
- `internal/lib/incident/warmer.go` - Cache warming
- `internal/lib/incident/rate_limiter.go` - OpenAI rate limiting
- `internal/cache/incident_store.go` - Processed incident storage
- `internal/cache/metrics.go` - Cache performance metrics
- `tests/contract/test_*` - Contract test files (7 files)
- `tests/integration/test_*` - Integration test files (4 files)  
- `tests/unit/test_*` - Unit test files (2 files)
- `tests/performance/test_*` - Performance test files (3 files)

**Modified Files**:
- `internal/cache/cache.go` - Extended with content-based methods
- `internal/config/config.go` - Added Store configuration
- `internal/services/roads.go` - Integrated background processing
- `cmd/server/main.go` - Background processing startup
- `cmd/test-caltrans/main.go` - Extended with content hash testing
- `CLAUDE.md` - Updated with configuration documentation

This task breakdown follows strict TDD principles with contract tests written first, implements the background processing system to achieve <200ms API responses, and maintains the existing library-first architecture while extending existing components.