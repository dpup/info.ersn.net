# ERSN API Server - Current Architecture Flow

## Information Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                              CLIENT REQUESTS                                            │
└─┬─────────────────────────────────────────────────────────────────────────────────────┬─┘
  │                                                                                     │
  │ HTTP GET /api/v1/roads          HTTP GET /api/v1/weather         HTTP GET /         │
  | ▼                               ▼                                ▼                  │
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                            PREFAB SERVER FRAMEWORK                                      │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐                          │
│  │   gRPC Gateway  │  │   gRPC Gateway  │  │  Homepage       │                          │
│  │   (HTTP→gRPC)   │  │   (HTTP→gRPC)   │  │  Handler        │                          │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘                          │
│           │                       │                       │                             │
│           ▼                       ▼                       ▼                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐                          │
│  │  RoadsService   │  │ WeatherService  │  │  Static HTML    │                          │
│  │                 │  │                 │  │  Response       │                          │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘                          │
└─┬─────────────────┬───────────────────────────────────────────────────────────────────┬─┘
  │                 │                                                                   │
  ▼                 ▼                                                                   │
┌─────────────────┐ ┌─────────────────────────────────────────────────────────────┐     │
│                 │ │              PERIODIC REFRESH SERVICE                       │     │
│  CACHE CHECK    │ │  ┌─────────────┐     Every 5 minutes:                       │     │
│                 │ │  │   Timer     │────▶ RoadsService.ListRoads()              │     │
│  ┌─────────────┐│ │  │  Goroutine  │     (Simulated API Request)                │     │
│  │ Fresh Data? ││ │  └─────────────┘                                            │     │
│  │             ││ │                                                             │     │
│  │ YES → Return││ │                                                             │     │
│  │ NO  → Fetch ││ └─────────────────────────────────────────────────────────────┘     │
│  └─────────────┘│                                                                     │
└─────────────────┘                                                                     │
          │                                                                             │
          ▼                                                                             │
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                          DATA REFRESH PIPELINE                                          │
│                                                                                         │
│  ┌─────────────────┐                         ┌──────────────────┐                       │
│  │ refreshRoadData │─────────────────────────│refreshWeatherData│                       │
│  │                 │                         │                  │                       │
│  │ For each road:  │                         │ For each loc:    │                       │
│  │ ▼               │                         │ ▼                │                       │
│  │ ┌─────────────┐ │                         │ ┌─────────────┐  │                       │
│  │ │processMonito│ │                         │ │getWeatherFor│  │                       │
│  │ │redRoad()    │ │                         │ │Location()   │  │                       │
│  │ └─────────────┘ │                         │ └─────────────┘  │                       │
│  └─────────────────┘                         └──────────────────┘                       │
│           │                                                                             │
│           ▼                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐ │
│  │                    EXTERNAL API CALLS                                           │ │
│  │                                                                                 │ │
│  │ ┌─────────────┐  ┌─────────────────┐  ┌─────────────────┐                     │ │
│  │ │   Google    │  │    Caltrans     │  │  OpenWeatherMap │                     │ │
│  │ │   Routes    │  │  KML Feeds      │  │      API        │                     │ │
│  │ │             │  │                 │  │                 │                     │ │
│  │ │ Traffic +   │  │ Lane Closures + │  │ Weather Data +  │                     │ │
│  │ │ Polylines   │  │ CHP Incidents   │  │ Alerts          │                     │ │
│  │ └─────────────┘  └─────────────────┘  └─────────────────┘                     │ │
│  │       │                   │                     │                             │ │
│  └───────┼───────────────────┼─────────────────────┼─────────────────────────────┘ │
│          │                   │                     │                               │
│          ▼                   ▼                     ▼                               │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐ │
│  │                    DATA PROCESSING PIPELINE                                     │ │
│  │                                                                                 │ │
│  │ Google Response    │    Caltrans KML      │    Weather Response                 │ │
│  │       │            │          │           │           │                        │ │
│  │       ▼            │          ▼           │           ▼                        │ │
│  │ ┌─────────────┐    │ ┌─────────────────┐  │  ┌─────────────────┐              │ │
│  │ │ Extract     │    │ │ Parse KML +     │  │  │ Format Weather  │              │ │
│  │ │ Polyline    │    │ │ Extract Coords  │  │  │ + Alerts        │              │ │
│  │ │ + Traffic   │    │ │                 │  │  │                 │              │ │
│  │ └─────────────┘    │ └─────────────────┘  │  └─────────────────┘              │ │
│  │       │            │          │           │                                    │ │
│  │       └────────────┼──────────▼           │                                    │ │
│  │                    │ ┌─────────────────┐  │                                    │ │
│  │                    │ │ Route-Aware     │  │                                    │ │
│  │                    │ │ Classification: │  │                                    │ │
│  │                    │ │ • OnRoute       │  │                                    │ │
│  │                    │ │ • Nearby        │  │                                    │ │
│  │                    │ │ • Distant       │  │                                    │ │
│  │                    │ └─────────────────┘  │                                    │ │
│  │                    │          │           │                                    │ │
│  │                    │          ▼           │                                    │ │
│  │                    │ ┌─────────────────┐  │                                    │ │
│  │                    │ │ Alert           │  │                                    │ │
│  │                    │ │ Enhancement     │  │                                    │ │
│  │                    │ │                 │  │                                    │ │
│  │                    │ │ ContentHasher   │  │                                    │ │
│  │                    │ │       │         │  │                                    │ │
│  │                    │ │       ▼         │  │                                    │ │
│  │                    │ │ ┌─────────────┐ │  │                                    │ │
│  │                    │ │ │ Check Cache │ │  │                                    │ │
│  │                    │ │ │ 24h TTL     │ │  │                                    │ │
│  │                    │ │ └─────────────┘ │  │                                    │ │
│  │                    │ │       │         │  │                                    │ │
│  │                    │ │       ▼         │  │                                    │ │
│  │                    │ │ ┌─────────────┐ │  │                                    │ │
│  │                    │ │ │ OpenAI API  │ │  │                                    │ │
│  │                    │ │ │ (if needed) │ │  │                                    │ │
│  │                    │ │ └─────────────┘ │  │                                    │ │
│  │                    │ └─────────────────┘  │                                    │ │
│  └─────────────────────────────────────────────────────────────────────────────────┘ │
│                                     │                                               │
│                                     ▼                                               │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐ │
│  │                              CACHE STORAGE                                     │ │
│  │                                                                                 │ │
│  │  ┌─────────────────┐    ┌─────────────────────┐    ┌─────────────────┐        │ │
│  │  │  roads:all      │    │ enhanced_alert:     │    │ Weather data    │        │ │
│  │  │  (5m TTL)       │    │ {content_hash}      │    │ (5m TTL)        │        │ │
│  │  │                 │    │ (24h TTL)           │    │                 │        │ │
│  │  │ • Road status   │    │                     │    │ • Current cond  │        │ │
│  │  │ • Traffic data  │    │ • AI enhanced       │    │ • Alerts        │        │ │
│  │  │ • Enhanced      │    │ • Structured desc   │    │                 │        │ │
│  │  │   alerts        │    │ • Impact/duration   │    │                 │        │ │
│  │  └─────────────────┘    └─────────────────────┘    └─────────────────┘        │ │
│  └─────────────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────────────────┘
```

## Key Components and Data Flows

### 1. Request Processing Flow
- **Entry**: HTTP requests → Prefab gRPC Gateway → gRPC Service methods
- **Cache Strategy**: Check cache first, refresh if stale, return data
- **Background Warmth**: Periodic refresh simulates requests to maintain cache

### 2. External API Integration ✅ **SIMPLIFIED CONFIG**
- **Google Routes**: Traffic conditions + polylines (API key via `PF__GOOGLE_ROUTES__API_KEY`)
- **Caltrans KML**: Real-time incidents parsed from XML feeds  
- **OpenWeather**: Current conditions and weather alerts (API key via `PF__OPENWEATHER__API_KEY`)
- **OpenAI**: AI enhancement for alerts (API key via `PF__OPENAI__API_KEY`)

### 3. Data Processing Layers
- **Route-Aware Classification**: Uses actual Google polylines to classify alerts as OnRoute/Nearby/Distant
- **AI Enhancement**: Content-based caching prevents duplicate OpenAI calls
- **Geographic Processing**: Polyline decoding and coordinate-based filtering

### 4. Caching Architecture
- **Single Cache Instance**: JSON-based with TTL support
- **Multi-Layer TTL**: 5m for API data, 24h for enhanced alerts
- **Content-Based Deduplication**: Prevents redundant AI processing

### 5. Configuration System ✅ **NEWLY SIMPLIFIED**
- **Unified Structure**: Single config.Config struct with top-level client configurations
- **Environment Mapping**: Prefab transforms `PF__SECTION__FIELD_NAME` → `section.fieldName`
- **Service Integration**: All services receive full config instead of sub-configs
- **Consistent Naming**: CamelCase throughout for predictable env var mapping

## Current Complexity Areas (Simplification Opportunities)

### 🔄 Route-Aware Processing Pipeline
**Current Flow:**
```
Raw KML → Parse → Extract Coords → Route Classification → AI Enhancement → Cache
```
**Complexity:** Multiple libraries (geo utils, route matcher, content hasher) handling overlapping concerns

### 🧠 AI Enhancement Chain  
**Current Flow:**
```
Content Hash → Cache Check → OpenAI API → Parse Response → Store Enhanced
```
**Complexity:** Separate caching logic from main cache, complex fallback handling

### 🌐 External API Client Patterns
**Similar Patterns:**
- Rate limiting and timeout handling
- JSON/XML parsing and error handling  
- Response caching and staleness detection

### 📊 Configuration Management ✅ **SIMPLIFIED**
**Unified Configuration Structure:**
- **Single Config Source**: Prefab YAML with environment variable mapping
- **Top-Level Client Configs**: GoogleRoutes, OpenAI, OpenWeather moved to root level  
- **CamelCase Consistency**: All fields use camelCase for Prefab env var transformation
- **Explicit Fields**: Replaced embedded RefreshConfig with explicit fields (koanf compatibility)
- **Environment Variables**: `PF__CLIENT__FIELD` → `client.field` mapping works correctly

**Configuration Reduction:**
- **Before**: 125 lines, nested structures, inconsistent naming
- **After**: ~98 lines, flat structure, consistent camelCase naming
- **Eliminated**: Obsolete StoreConfig, unused chain_controls section

