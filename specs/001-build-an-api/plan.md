# Implementation Plan: Live Data API Server

**Branch**: `001-build-an-api` | **Date**: 2025-09-09 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/Users/pupius/Dropbox/Projects/Sites/info.ersn.net/specs/001-build-an-api/spec.md`

## Execution Flow (/plan command scope)
```
1. Load feature spec from Input path
   → If not found: ERROR "No feature spec at {path}"
2. Fill Technical Context (scan for NEEDS CLARIFICATION)
   → Detect Project Type from context (web=frontend+backend, mobile=app+api)
   → Set Structure Decision based on project type
3. Evaluate Constitution Check section below
   → If violations exist: Document in Complexity Tracking
   → If no justification possible: ERROR "Simplify approach first"
   → Update Progress Tracking: Initial Constitution Check
4. Execute Phase 0 → research.md
   → If NEEDS CLARIFICATION remain: ERROR "Resolve unknowns"
5. Execute Phase 1 → contracts, data-model.md, quickstart.md, agent-specific template file (e.g., `CLAUDE.md` for Claude Code, `.github/copilot-instructions.md` for GitHub Copilot, or `GEMINI.md` for Gemini CLI).
6. Re-evaluate Constitution Check section
   → If new violations: Refactor design, return to Phase 1
   → Update Progress Tracking: Post-Design Constitution Check
7. Plan Phase 2 → Describe task generation approach (DO NOT create tasks.md)
8. STOP - Ready for /tasks command
```

**IMPORTANT**: The /plan command STOPS at step 7. Phases 2-4 are executed by other commands:
- Phase 2: /tasks command creates tasks.md
- Phase 3-4: Implementation execution (manual or via tools)

## Summary
Build a Live Data API Server that provides REST endpoints for road conditions and weather data to a static website. The server will use Go + gRPC + gRPC Gateway with the Prefab framework for orchestration. Data will be cached in-memory with configurable refresh intervals (5min default) from external APIs: Google Routes API for road conditions and OpenWeatherMap for weather data.

## Technical Context
**Language/Version**: Go 1.21+  
**Primary Dependencies**: gRPC, gRPC Gateway, Prefab framework (github.com/dpup/prefab), Protocol Buffers  
**Storage**: In-memory caching (no persistent storage required)  
**Testing**: Go testing framework with testify, contract tests for gRPC services  
**Target Platform**: Linux/macOS server, containerizable
**Project Type**: single (API server only - backend service)  
**Performance Goals**: Handle low concurrent usage, <5 second response times, 5-minute data refresh cycles  
**Constraints**: Single instance deployment, CORS-enabled for static website, no authentication required  
**Scale/Scope**: Small-scale personal/regional use, handful of routes and weather locations, minimal concurrent users

**Architecture Approach**: Use Go + gRPC + gRPC Gateway with Prefab framework for server orchestration and configuration. Proto-first design with service definitions, abstracted clients for Google Routes API and OpenWeatherMap, in-memory caching with configurable refresh intervals.

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Simplicity**:
- Projects: 1 (API server only)
- Using framework directly? Yes (Prefab framework used directly, no wrappers)
- Single data model? Yes (protobuf definitions serve as both API contract and data model)
- Avoiding patterns? Yes (direct client calls, no Repository pattern needed for simple caching)

**Architecture**:
- EVERY feature as library? Yes - external clients and parsers as libraries
- Libraries listed: 
  - `clients/google` - Google Routes API client with rate limiting
  - `clients/caltrans` - Caltrans KML feed parser and data extractor
  - `clients/weather` - OpenWeatherMap API client
  - `cache` - In-memory caching with TTL and refresh logic
  - `services` - gRPC service implementations (roads, weather)
- CLI per library: `cmd/test-google`, `cmd/test-caltrans`, `cmd/test-weather`
- Library docs: Each client includes usage examples and configuration docs

**Testing (NON-NEGOTIABLE)**:
- RED-GREEN-Refactor cycle enforced? YES - contract tests written first, must fail before implementation
- Git commits show tests before implementation? YES - commit order: tests → implementation → refactor
- Order: Contract→Integration→E2E→Unit strictly followed? YES - gRPC contracts → API integration → end-to-end → unit tests
- Real dependencies used? YES - actual external API calls in integration tests, rate-limited test suite
- Integration tests for: new libraries (clients), contract changes (proto updates), shared schemas (data model changes)
- FORBIDDEN: Implementation before test, skipping RED phase

**Observability**:
- Structured logging included? YES - structured JSON logs via Prefab framework
- Frontend logs → backend? N/A - static website logs separately
- Error context sufficient? YES - client errors, API failures, cache misses all logged with context

**Versioning**:
- Version number assigned? v0.1.0 (MAJOR.MINOR.BUILD)
- BUILD increments on every change? YES - automated via Makefile and CI
- Breaking changes handled? YES - parallel proto versions, migration docs in quickstart.md

## Project Structure

### Documentation (this feature)
```
specs/[###-feature]/
├── plan.md                    # This file (/plan command output)
├── research.md                # Phase 0 output (/plan command)
├── data-model.md              # Phase 1 output (/plan command)
├── quickstart.md              # Phase 1 output (/plan command)
├── contracts/                 # Phase 1 output (/plan command)
├── implementation-guide.md    # Phase 1 output (/plan command) - Cross-references and examples
└── tasks.md                   # Phase 2 output (/tasks command - NOT created by /plan)
```

### Source Code (repository root)
```
# Option 1: Single project (DEFAULT)
src/
├── models/
├── services/
├── cli/
└── lib/

tests/
├── contract/
├── integration/
└── unit/

# Option 2: Web application (when "frontend" + "backend" detected)
backend/
├── src/
│   ├── models/
│   ├── services/
│   └── api/
└── tests/

frontend/
├── src/
│   ├── components/
│   ├── pages/
│   └── services/
└── tests/

# Option 3: Mobile + API (when "iOS/Android" detected)
api/
└── [same as backend above]

ios/ or android/
└── [platform-specific structure]
```

**Structure Decision**: Option 1 (Single project) - API server only with no frontend components

## Phase 0: Outline & Research
1. **Extract unknowns from Technical Context** above:
   - For each NEEDS CLARIFICATION → research task
   - For each dependency → best practices task
   - For each integration → patterns task

2. **Generate and dispatch research agents**:
   ```
   For each unknown in Technical Context:
     Task: "Research {unknown} for {feature context}"
   For each technology choice:
     Task: "Find best practices for {tech} in {domain}"
   ```

3. **Consolidate findings** in `research.md` using format:
   - Decision: [what was chosen]
   - Rationale: [why chosen]
   - Alternatives considered: [what else evaluated]

**Output**: research.md with all NEEDS CLARIFICATION resolved

## Phase 1: Design & Contracts
*Prerequisites: research.md complete*

1. **Extract entities from feature spec** → `data-model.md`:
   - Entity name, fields, relationships
   - Validation rules from requirements
   - State transitions if applicable

2. **Generate API contracts** from functional requirements:
   - For each user action → endpoint
   - Use standard REST/GraphQL patterns
   - Output OpenAPI/GraphQL schema to `/contracts/`

3. **Generate contract tests** from contracts:
   - One test file per endpoint
   - Assert request/response schemas
   - Tests must fail (no implementation yet)

4. **Extract test scenarios** from user stories:
   - Each story → integration test scenario
   - Quickstart test = story validation steps

5. **Update agent file incrementally** (O(1) operation):
   - Run `/scripts/update-agent-context.sh [claude|gemini|copilot]` for your AI assistant
   - If exists: Add only NEW tech from current plan
   - Preserve manual additions between markers
   - Update recent changes (keep last 3)
   - Keep under 150 lines for token efficiency
   - Output to repository root

**Output**: data-model.md, /contracts/*, failing tests, quickstart.md, agent-specific file

## Phase 2: Task Planning Approach
*This section describes what the /tasks command will do - DO NOT execute during /plan*

**Implementation Sequence Overview**:
```
Foundation → External Clients → Services → Integration → Deployment
```

**Detailed Task Generation Strategy**:

**Foundation Tasks** (Tasks 1-8):
1. **Setup Project Structure**: Create Go module, Makefile, basic directories
   - Reference: `research.md` project structure, `quickstart.md` setup steps
2. **Generate Protobuf Code**: Set up proto generation pipeline
   - Reference: `contracts/roads.proto`, `contracts/weather.proto`
3. **Configuration Framework**: Implement config loading with Prefab
   - Reference: `research.md` configuration structure
4. **Basic Caching Infrastructure**: In-memory cache with TTL
   - Reference: `data-model.md` Cache Entry entity

**External Client Tasks** (Tasks 9-20) - Can run in parallel [P]:
5. **[P] Google Routes Client + Tests**: Implement coordinate-based API client
   - Reference: `research.md` Google Routes API section, `data-model.md` TrafficCondition
6. **[P] Caltrans KML Parser + Tests**: Implement KML feed parsing and geographic filtering
   - Reference: `research.md` Caltrans integration, `data-model.md` Route status fields
7. **[P] OpenWeatherMap Client + Tests**: Implement weather API client
   - Reference: `data-model.md` WeatherData entity
8. **[P] CLI Testing Tools**: Build `test-google`, `test-caltrans`, `test-weather`
   - Reference: `research.md` CLI tool features, `quickstart.md` testing commands

**Service Implementation Tasks** (Tasks 21-28):
9. **RoadsService Contract Tests**: Write failing gRPC service tests
   - Reference: `contracts/roads.proto` service definitions
10. **RoadsService Implementation**: Implement gRPC methods using clients and cache
    - Reference: `data-model.md` Route entity, external client interfaces
11. **WeatherService Contract Tests**: Write failing gRPC service tests
    - Reference: `contracts/weather.proto` service definitions
12. **WeatherService Implementation**: Implement gRPC methods
    - Reference: `data-model.md` WeatherData entity

**Integration Tasks** (Tasks 29-35):
13. **Server Integration**: Wire services with Prefab framework
    - Reference: `research.md` Prefab integration patterns
14. **End-to-End Tests**: Test full API workflows
    - Reference: `quickstart.md` API usage examples
15. **Performance Validation**: Verify <5s response times, caching behavior
    - Reference: Plan technical constraints

**Cross-Reference Mapping**:
- **Data Models** → `data-model.md` entities map to proto messages in `contracts/`
- **API Clients** → Implementation details in `research.md` external API sections
- **Configuration** → Structure defined in `research.md`, examples in `quickstart.md`
- **Testing** → CLI tools documented in `quickstart.md` troubleshooting section

**Ordering Strategy**:
- TDD order: Contract tests → Implementation → Integration tests
- Dependency order: Foundation → Clients → Services → Integration
- Parallel execution: External clients can be built simultaneously
- References: Each task includes specific documentation sections to consult

**Estimated Output**: 35 numbered, ordered tasks in tasks.md with explicit cross-references

**IMPORTANT**: This phase is executed by the /tasks command, NOT by /plan

## Phase 3+: Future Implementation
*These phases are beyond the scope of the /plan command*

**Phase 3**: Task execution (/tasks command creates tasks.md)  
**Phase 4**: Implementation (execute tasks.md following constitutional principles)  
**Phase 5**: Validation (run tests, execute quickstart.md, performance validation)

## Complexity Tracking
*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |


## Progress Tracking
*This checklist is updated during execution flow*

**Phase Status**:
- [x] Phase 0: Research complete (/plan command)
- [x] Phase 1: Design complete (/plan command)
- [x] Phase 2: Task planning complete (/plan command - describe approach only)
- [ ] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS
- [x] Post-Design Constitution Check: PASS
- [x] All NEEDS CLARIFICATION resolved
- [ ] Complexity deviations documented

---
*Based on Constitution v2.1.1 - See `/memory/constitution.md`*