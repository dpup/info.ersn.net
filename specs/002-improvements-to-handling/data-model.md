# Data Model: Smart Route-Relevant Alert Filtering

**Feature**: 002-improvements-to-handling | **Date**: 2025-09-11  
**Source**: Entities extracted from spec.md functional requirements

## Core Entities

### RoadAlert (Updated Structure)
**Purpose**: Enhanced road alert with AI processing and route classification (replaces existing RoadAlert)

**Fields**:
- `ID`: Unique identifier for alert
- `Source`: Data source identifier ("caltrans", "google", "weather")  
- `Location`: Geographic coordinates (latitude, longitude)
- `Classification`: Route relationship ("on_route", "nearby") // NEW
- `OriginalDescription`: Raw description from source // NEW
- `Description`: AI-processed, human-readable description (replaces existing description field)
- `CondensedSummary`: Short format for mobile display // NEW
- `Severity`: Impact level ("info", "warning", "critical")
- `Type`: Alert category ("closure", "incident", "construction", "weather")
- `TimeReported`: When incident was first reported
- `LastUpdated`: When alert was last refreshed
- `Metadata`: Key-value map for AI-extracted additional data // NEW
- `AffectedPolyline`: Encoded polyline if incident covers route segment // NEW

**Validation Rules**:
- Location coordinates must be valid lat/lng (-90 to 90, -180 to 180)
- Classification must be one of defined enum values
- TimeReported cannot be in the future
- Severity and Type must match predefined categories

**State Transitions**:
- Raw → AI-Enhanced: When OpenAI processing completes
- Unclassified → Route-Classified: When route matching runs (point or polyline-based)
- Active → Stale: When LastUpdated exceeds threshold

**Classification Logic**:
- **Point-based**: For incidents with single location (accidents, hazards)
  - ON_ROUTE: Point within 100m of route polyline
  - NEARBY: Point within configured threshold (default 10 miles) but not on route
- **Polyline-based**: For incidents with route coverage (closures, construction)
  - ON_ROUTE: Incident polyline overlaps >10% with route polyline
  - NEARBY: Incident polyline within threshold distance but <10% overlap

### RouteSegment (Extended)
**Purpose**: Route definition with geometry for precise alert matching

**Fields**:
- `ID`: Route identifier (e.g., "hwy4-angels-murphys")
- `Name`: Highway name (e.g., "Hwy 4")
- `Section`: Descriptive section (e.g., "Angels Camp to Murphys")
- `Origin`: Start coordinates
- `Destination`: End coordinates  
- `Polyline`: Google Routes encoded polyline string
- `DecodedPoints`: Array of lat/lng points from polyline
- `MaxDistance`: Distance threshold for "nearby" classification (default 10 miles)

**Validation Rules**:
- Origin and Destination must be different points
- Polyline must decode to valid coordinate sequence
- DecodedPoints must have minimum 2 points
- MaxDistance must be positive value

**Relationships**:
- One-to-Many with GeoAlert (alerts can affect multiple routes)
- Contains geometry data for precise distance calculations

### AIProcessedDescription (Structured Output)  
**Purpose**: AI-processed incident information in standardized format (replaces raw description field)

**Standard Fields**:
- `TimeReported`: Parsed timestamp from description
- `Details`: Core incident information without jargon
- `Location`: Human-readable location description
- `LastUpdate`: Most recent update timestamp
- `Impact`: Expected traffic impact ("none", "light", "moderate", "severe")
- `Duration`: Expected duration ("unknown", "< 1 hour", "several hours", "ongoing")

**Dynamic Fields**:
- Key-value map for incident-specific information
- Examples: `{"visibility": "not visible from roadway"}`, `{"lanes_affected": "2 of 4"}`
- Preserved from original descriptions when AI identifies useful details

**Condensed Format Template**:
```
{Highway} – {Location}
{Brief description}, {status} ({time}).
```

**Validation Rules**:
- TimeReported must be parseable datetime or null
- Impact and Duration must be from predefined categories
- Dynamic fields must have string keys and values
- Condensed format must be under 200 characters

### AlertClassification (Enum)
**Purpose**: Categorize alert relationship to monitored routes

**Values**:
- `ON_ROUTE`: Incident directly affects route path (distance < 0.1 miles from polyline)
- `NEARBY`: Incident in surrounding area (distance < MaxDistance from route)
- `DISTANT`: Incident too far from route (filtered out, not returned in API)

**Classification Logic**:
- Calculate minimum distance from incident to route polyline
- ON_ROUTE: Point intersects or very close to route geometry
- NEARBY: Point within configured threshold but not on route
- DISTANT: Point beyond threshold, excluded from responses

## Data Flow

### Input Processing
1. **Raw Caltrans Data** → CaltransToGeoAlert adapter → GeoAlert
2. **Google Routes Data** → Extract polyline → Update RouteSegment geometry
3. **GeoAlert** → RouteClassifier → Classification assigned
4. **GeoAlert** → AlertEnhancer (OpenAI) → EnhancedDescription populated

### Storage Strategy  
- **In-Memory Only**: No persistent storage, cache-based architecture
- **Cache Tiers**: 
  - Raw alerts (5-minute TTL)
  - Enhanced alerts (15-minute TTL) 
  - Route geometry (1-hour TTL, rarely changes)

### API Response Mapping
- Filter by Classification (exclude DISTANT)
- Group by RouteSegment ID
- Sort by Severity (ON_ROUTE alerts first)
- Include EnhancedDescription for user display

## Validation & Constraints

### Geographic Constraints
- Alert coordinates must be within reasonable bounds (Continental US focus)
- Route polylines must have sufficient point density for accurate distance calculations
- Distance calculations use great-circle (spherical) geometry

### Processing Constraints  
- OpenAI enhancement must complete within 10 seconds or timeout
- Route classification must complete within 100ms per alert
- Total processing pipeline must not block API responses

### API Constraints
- Enhanced descriptions limited to 500 characters max
- Dynamic metadata limited to 10 key-value pairs per alert
- Response payload size limited to 1MB total

## Extension Points

### Future Data Sources
- Weather alerts: Adapt to GeoAlert interface via WeatherToGeoAlert
- User reports: Community-submitted incidents via UserToGeoAlert  
- DOT cameras: Computer vision alerts via VisionToGeoAlert

### Enhancement Strategies
- Multiple AI providers: Strategy pattern for different enhancement services
- Caching variations: Redis backend for distributed deployments
- Real-time updates: WebSocket push for critical alerts

### Classification Refinements
- Time-based relevance: Weight recent incidents higher
- Route direction sensitivity: Different thresholds for opposing directions
- Incident type weighting: Construction vs accidents vs weather