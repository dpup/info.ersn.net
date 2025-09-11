# Implementation Plan: Smart Route-Relevant Alert Filtering

**Branch**: `002-improvements-to-handling` | **Date**: 2025-09-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-improvements-to-handling/spec.md`

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
**Primary Requirement**: Filter Caltrans alerts to show only route-relevant incidents with user-friendly descriptions, classified as "On Route" vs "Nearby".

**Technical Approach**: Use Google Routes API polylines to compare against Caltrans incident locations, implement abstract utility functions for geo-comparisons, and integrate OpenAI API for description enhancement. Map both Google and Caltrans data to common structure for unified processing.

## Technical Context
**Language/Version**: Go 1.21+  
**Primary Dependencies**: gRPC, gRPC Gateway, Prefab framework, Google Routes API, OpenAI API  
**Storage**: In-memory caching with TTL (no persistent storage required)  
**Testing**: Go testing framework with testify, contract tests for gRPC services  
**Target Platform**: Linux/macOS server, containerizable
**Project Type**: single (gRPC API server with HTTP gateway)  
**Performance Goals**: <2s response time for route alerts, 5-minute cache refresh intervals  
**Constraints**: 10-mile configurable distance threshold, OpenAI API rate limits, existing Google Routes integration  
**Scale/Scope**: 3 monitored route segments, ~1000 daily requests, enhanced alert filtering and description processing

**User-Provided Implementation Details**:
- Leverage existing Google Routes API polylines for route geometry comparison
- Implement abstract utility functions for location/geo-line comparisons  
- Map Google and Caltrans data to common structure for unified processing
- Integrate OpenAI API for description enhancement with structured output
- Support configurable distance thresholds and fuzzy matching
- Generate condensed, user-friendly alert formats

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Simplicity**:
- Projects: 1 (existing gRPC API server)
- Using framework directly? YES (direct gRPC, no wrappers)
- Single data model? YES (common geo-alert structure)
- Avoiding patterns? YES (direct service integration, no unnecessary abstractions)

**Architecture**:
- EVERY feature as library? YES (geo-utils, alert-enhancer, route-matcher libraries)
- Libraries listed: 
  - `geo-utils`: Abstract utilities for location/polyline comparisons
  - `alert-enhancer`: OpenAI integration for description processing  
  - `route-matcher`: Core logic for classifying alerts as On Route/Nearby
- CLI per library: YES (test-geo-utils, test-alert-enhancer, test-route-matcher)
- Library docs: YES (llms.txt format planned)

**Testing (NON-NEGOTIABLE)**:
- RED-GREEN-Refactor cycle enforced? YES (tests written first, must fail)
- Git commits show tests before implementation? YES (enforced in workflow)
- Order: Contract→Integration→E2E→Unit strictly followed? YES
- Real dependencies used? YES (actual OpenAI API, real KML feeds for integration tests)
- Integration tests for: new libraries, OpenAI contract changes, enhanced alert schemas? YES
- FORBIDDEN: Implementation before test, skipping RED phase

**Observability**:
- Structured logging included? YES (existing Prefab framework logging)
- Frontend logs → backend? N/A (API server only)
- Error context sufficient? YES (enhanced with geo-matching context)

**Versioning**:
- Version number assigned? YES (increment existing API version)
- BUILD increments on every change? YES
- Breaking changes handled? YES (new optional alert fields, backward compatible)

## Project Structure

### Documentation (this feature)
```
specs/[###-feature]/
├── plan.md              # This file (/plan command output)
├── research.md          # Phase 0 output (/plan command)
├── data-model.md        # Phase 1 output (/plan command)
├── quickstart.md        # Phase 1 output (/plan command)
├── contracts/           # Phase 1 output (/plan command)
└── tasks.md             # Phase 2 output (/tasks command - NOT created by /plan)
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

**Structure Decision**: Option 1 (Single project) - Adding libraries to existing gRPC API server

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

**Task Generation Strategy**:
- Load `/templates/tasks-template.md` as base
- Generate tasks from Phase 1 design docs (contracts, data model, quickstart)
- Three main libraries: geo-utils, alert-enhancer, route-matcher → contract test tasks [P]
- Each library interface → implementation tasks following TDD
- Integration tasks for combining libraries in existing roads service
- User story validation tasks from quickstart.md scenarios

**Ordering Strategy**:
- TDD order enforced: Contract tests → failing implementation → passing implementation
- Library dependency order: geo-utils (foundation) → route-matcher (uses geo) → alert-enhancer (independent) → integration
- Mark [P] for parallel execution where libraries are independent
- OpenAI integration last (requires all other components)

**Specific Task Categories**:
1. **Contract Tests** [P]: geo-utils, alert-enhancer, route-matcher interfaces
2. **Core Library Implementation**: TDD for each library with CLI tools
3. **Integration Tasks**: Combine libraries in existing roads service  
4. **Enhancement Tasks**: Modify existing Caltrans parser and roads service
5. **Configuration Tasks**: Add OpenAI API key and distance threshold configs
6. **Validation Tasks**: Execute quickstart.md test scenarios

**Estimated Output**: 28-32 numbered, ordered tasks in tasks.md

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
- [x] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS
- [x] Post-Design Constitution Check: PASS
- [x] All NEEDS CLARIFICATION resolved
- [x] Complexity deviations documented (none required)

---
*Based on Constitution v2.1.1 - See `/memory/constitution.md`*