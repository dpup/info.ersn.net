# ERSN API Server - Current Architecture Flow

## Information Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                              CLIENT REQUESTS                                               │
└─┬─────────────────────────────────────────────────────────────────────────────────────┬─┘
  │                                                                                       │
  │ HTTP GET /api/v1/roads          HTTP GET /api/v1/weather         HTTP GET /         │
  ▼                                 ▼                                ▼                   │
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                            PREFAB SERVER FRAMEWORK                                     │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐                       │
│  │   gRPC Gateway  │  │   gRPC Gateway  │  │  Homepage       │                       │
│  │   (HTTP→gRPC)   │  │   (HTTP→gRPC)   │  │  Handler        │                       │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘                       │
│           │                       │                       │                           │
│           ▼                       ▼                       ▼                           │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐                       │
│  │  RoadsService   │  │ WeatherService  │  │  Static HTML    │                       │
│  │                 │  │                 │  │  Response       │                       │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘                       │
└─┬─────────────────┬─────────────────────────────────────────────────────────────────┬─┘
  │                 │                                                                   │
  ▼                 ▼                                                                   │
┌─────────────────┐ ┌─────────────────────────────────────────────────────────────┐   │
│                 │ │              PERIODIC REFRESH SERVICE                       │   │
│  CACHE CHECK    │ │  ┌─────────────┐     Every 5 minutes:                       │   │
│                 │ │  │   Timer     │────▶ RoadsService.ListRoads()             │   │
│  ┌─────────────┐│ │  │  Goroutine  │     (Simulated API Request)                │   │
│  │ Fresh Data? ││ │  └─────────────┘                                            │   │
│  │             ││ │                                                              │   │
│  │ YES → Return││ │                                                              │   │
│  │ NO  → Fetch ││ └─────────────────────────────────────────────────────────────┘   │
│  └─────────────┘│                                                                   │
└─────────────────┘                                                                   │
          │                                                                           │
          ▼                                                                           │
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                          DATA REFRESH PIPELINE                                         │
│                                                                                         │
│  ┌─────────────────┐                        ┌─────────────────┐                       │
│  │ refreshRoadData │─────────────────────────│refreshWeatherData│                      │
│  │                 │                        │                  │                      │
│  │ For each road:  │                        │ For each loc:    │                      │
│  │ ▼               │                        │ ▼                │                      │
│  │ ┌─────────────┐ │                        │ ┌─────────────┐  │                      │
│  │ │processMonito│ │                        │ │getWeatherFor│  │                      │
│  │ │redRoad()    │ │                        │ │Location()   │  │                      │
│  │ └─────────────┘ │                        │ └─────────────┘  │                      │
│  └─────────────────┘                        └─────────────────┘                       │
│           │                                                                           │
│           ▼                                                                           │
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

### 2. External API Integration
- **Google Routes**: Traffic conditions + encoded polylines for route geometry
- **Caltrans KML**: Real-time incidents parsed from XML feeds  
- **OpenWeather**: Current conditions and weather alerts

### 3. Data Processing Layers
- **Route-Aware Classification**: Uses actual Google polylines to classify alerts as OnRoute/Nearby/Distant
- **AI Enhancement**: Content-based caching prevents duplicate OpenAI calls
- **Geographic Processing**: Polyline decoding and coordinate-based filtering

### 4. Caching Architecture
- **Single Cache Instance**: JSON-based with TTL support
- **Multi-Layer TTL**: 5m for API data, 24h for enhanced alerts
- **Content-Based Deduplication**: Prevents redundant AI processing

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

### 📊 Configuration Management
**Multiple Config Layers:**
- Prefab YAML configuration
- Environment variable overrides
- Per-service configuration structs
- Runtime refresh interval management

---

**Next Steps for Simplification:**
1. Consolidate geographic processing libraries
2. Merge AI enhancement caching with main cache strategy  
3. Create unified external API client pattern
4. Simplify configuration structure