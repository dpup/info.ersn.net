# Data Model - Live Data API Server

## Core Entities

### Route
Represents a monitored road/highway route with current conditions from multiple data sources.

**Fields**:
- `id`: string - Unique identifier for the route
- `name`: string - Human-readable route name (e.g., "I-5 Seattle to Portland")
- `origin`: Coordinates - Starting point for route (lat/lon for Google Routes API)
- `destination`: Coordinates - Ending point for route (lat/lon for Google Routes API)
- `status`: RouteStatus - Current operational status (from Caltrans API)
- `traffic_condition`: TrafficCondition - Current traffic state (from Google Routes API)
- `chain_control`: ChainControlStatus - Chain/traction tire requirements (from Caltrans API)
- `alerts`: []RouteAlert - Active alerts and warnings (combined from multiple sources)
- `last_updated`: timestamp - When data was last refreshed

**Validation Rules**:
- `id` must be non-empty and unique across all routes
- `name` must be non-empty and descriptive
- `status` must be valid enum value
- `last_updated` must be recent (within stale threshold)

### TrafficCondition  
Current traffic state for a route, processed from Google Routes API v2.

**Fields** (consistent units - seconds/meters):
- `route_id`: string - Internal route identifier
- `duration_seconds`: int - Total travel time in seconds
- `distance_meters`: int - Route distance in meters
- `congestion_level`: CongestionLevel - Traffic density level (CLEAR, LIGHT, MODERATE, HEAVY, SEVERE)
- `delay_seconds`: int - Additional time due to traffic in seconds (0 = no delays)

**Processing Logic** (from Google Routes API):
```
Google Routes API Response → Server Processing → API Response
"duration": "450s"         → parseDuration() → duration_seconds: 450
"distanceMeters": 1500     → direct copy    → distance_meters: 1500
"speedReadingIntervals"    → analyzeCongestion() → congestion_level: LIGHT
Historical comparison      → calculateDelay() → delay_seconds: 0
```

**Validation Rules**:
- `duration_seconds` must be positive
- `distance_meters` must be positive  
- `congestion_level` must be valid enum value
- `delay_seconds` must be non-negative

### RouteAlert
Warnings, closures, or important notices affecting route conditions from multiple sources.

**Fields**:
- `id`: string - Unique alert identifier
- `source`: AlertSource - Data source (CALTRANS_KML, GOOGLE_ROUTES)
- `type`: AlertType - Category of alert
- `severity`: AlertSeverity - Urgency level  
- `title`: string - Brief alert description (from KML `name` or generated)
- `description`: string - Detailed alert information (from KML `description` or API)
- `coordinates`: Coordinates - Geographic location of alert
- `start_time`: timestamp - When alert became active
- `end_time`: timestamp - Expected resolution time (optional)
- `affected_segments`: []string - Route segments impacted
- `raw_data`: string - Original HTML/JSON data for debugging

### CaltransIncident
Raw incident data parsed from Caltrans KML feeds.

**Fields** (mapped from Caltrans KML structure):
- `feed_type`: CaltransFeedType - Which feed (CHAIN_CONTROL, LANE_CLOSURE, CHP_INCIDENT)
- `name`: string - Placemark name from KML
- `description_html`: string - Raw HTML description from KML CDATA
- `description_text`: string - Extracted text from HTML
- `style_url`: string - KML style reference (indicates closure type)
- `coordinates`: Coordinates - Point location from KML
- `parsed_status`: string - Extracted status from description
- `parsed_dates`: []string - Extracted dates/times from description
- `last_fetched`: timestamp - When KML was last fetched

**Caltrans KML Structure**:
```xml
<Placemark>
  <name>US-395 at Conway Summit</name>           → name
  <description><![CDATA[<div>HTML content</div>]]> → description_html
  <styleUrl>#full-closure</styleUrl>             → style_url
  <Point>
    <coordinates>-119.123,38.456,0</coordinates> → coordinates
  </Point>
</Placemark>
```

**CaltransFeedType Enum**:
- `CHAIN_CONTROL` - Chain control requirements (cc.kml)
- `LANE_CLOSURE` - Lane closures and restrictions (lcs2way.kml)  
- `CHP_INCIDENT` - CHP incident reports (chp-only.kml)

**Validation Rules**:
- `type` must be valid enum (CLOSURE, CONSTRUCTION, INCIDENT, WEATHER)
- `severity` must be valid enum (INFO, WARNING, CRITICAL)
- `title` and `description` must be non-empty
- `coordinates` must be valid WGS84 decimal degrees
- `description_html` may contain HTML requiring sanitization

### WeatherData
Current weather conditions for a monitored location, processed from OpenWeatherMap API.

**Fields** (consistent units - Celsius/meters/seconds):
- `location_id`: string - Internal location identifier
- `location_name`: string - Location name for display
- `coordinates`: Coordinates - Lat/lon coordinates
- `weather_main`: string - Main weather category ("Clear", "Rain", "Snow", etc.)
- `weather_description`: string - Detailed description ("light rain", "clear sky", etc.)
- `weather_icon`: string - Icon code for website display
- `temperature_celsius`: float - Temperature in Celsius
- `feels_like_celsius`: float - Feels like temperature in Celsius
- `humidity_percent`: int - Humidity percentage (0-100)
- `wind_speed_ms`: float - Wind speed in meters per second
- `wind_direction_degrees`: int - Wind direction in degrees (0-360, 0=North)
- `visibility_meters`: int - Visibility distance in meters
- `alerts`: []WeatherAlert - Active weather alerts
- `last_updated`: timestamp - When data was cached

**OpenWeatherMap API Mapping**:
```json
{
  "coord": {"lat": 33.44, "lon": -94.04},     → coordinates
  "weather": [{
    "main": "Rain",                           → weather_main
    "description": "moderate rain",           → weather_description  
    "icon": "10d"                            → weather_icon
  }],
  "main": {
    "temp": 298.48,                          → temperature_kelvin
    "feels_like": 298.74,                    → feels_like_f (converted)
    "pressure": 1015,                        → pressure_mb
    "humidity": 64                           → humidity_percent
  },
  "visibility": 10000,                       → visibility_meters
  "wind": {
    "speed": 0.62,                          → wind_speed_ms
    "deg": 349                              → wind_direction_deg
  },
  "clouds": {"all": 100},                   → cloud_cover_percent
  "dt": 1661870592                          → data_timestamp
}
```

**Validation Rules**:
- Coordinates must have valid latitude (-90 to 90) and longitude (-180 to 180)
- `temperature_kelvin` must be > 0 (absolute zero check)
- `humidity_percent` must be 0-100
- `wind_direction_deg` must be 0-360
- `cloud_cover_percent` must be 0-100

### WeatherAlert
Weather warnings, watches, or advisories from OpenWeatherMap One Call API 3.0.

**Fields** (mapped from OpenWeatherMap alerts response):
- `id`: string - Generated unique identifier (hash of sender + event + start time)
- `sender_name`: string - Alert issuing organization (from `sender_name`)
- `event`: string - Alert event type (from `event`)
- `start_timestamp`: int - Unix timestamp when effective (from `start`)
- `end_timestamp`: int - Unix timestamp when expires (from `end`)
- `description`: string - Detailed alert description (from `description`)
- `tags`: []string - Alert categories (from `tags` array)
- `start_time`: timestamp - Converted start time for display
- `end_time`: timestamp - Converted end time for display

**OpenWeatherMap API Mapping**:
```json
{
  "alerts": [{
    "sender_name": "NWS Tulsa (Eastern Oklahoma)",  → sender_name
    "event": "Heat Advisory",                       → event
    "start": 1597341600,                           → start_timestamp
    "end": 1597366800,                             → end_timestamp
    "description": "...HEAT ADVISORY REMAINS...",   → description
    "tags": ["extreme temperature value"]          → tags
  }]
}
```

**OpenWeatherMap Alert Tags** (14 categories):
- `extreme temperature value` (hot/cold)
- `fog`, `high wind`, `thunderstorms`, `tornado`
- `hurricane/typhoon`, `snow`, `ice`, `rain`
- `coastal event`, `volcano`, `tsunami`, `other`

**Validation Rules**:
- `sender_name` must be non-empty
- `event` must be non-empty
- `start_timestamp` must be valid Unix timestamp
- `end_timestamp` must be > `start_timestamp`
- `tags` array may be empty but should contain recognized categories

### SpeedReading
Granular traffic speed data for specific points along a route from Google Routes API.

**Fields** (mapped from Google Routes speedReadingIntervals):
- `start_polyline_point_index`: int - Starting point in polyline (from API)
- `end_polyline_point_index`: int - Ending point in polyline (from API)  
- `speed_category`: SpeedCategory - Traffic speed classification (from API)
- `coordinates_start`: Coordinates - Decoded geographic position of start point
- `coordinates_end`: Coordinates - Decoded geographic position of end point

**Google Routes API Mapping**:
```json
{
  "speedReadingIntervals": [{
    "startPolylinePointIndex": 0,        → start_polyline_point_index
    "endPolylinePointIndex": 10,         → end_polyline_point_index
    "speed": "NORMAL"                    → speed_category
  }]
}
```

**SpeedCategory Enum** (from Google Routes API):
- `NORMAL` - Normal traffic flow
- `SLOW` - Slower than normal traffic
- `TRAFFIC_JAM` - Heavy traffic or traffic jam

**Validation Rules**:
- `end_polyline_point_index` must be > `start_polyline_point_index`
- `speed_category` must be valid Google Routes enum value
- Coordinates calculated by decoding polyline at specified indices

### Cache Entry
Internal cache management for data freshness.

**Fields**:
- `key`: string - Cache key identifier
- `data`: []byte - Serialized cached data
- `created_at`: timestamp - When entry was cached
- `expires_at`: timestamp - When entry becomes stale
- `refresh_interval`: duration - How often to refresh
- `source`: string - Original data source identifier

**Validation Rules**:
- `key` must be non-empty and unique
- `expires_at` must be after `created_at`
- `refresh_interval` must be positive duration

## Enumerations

### RouteStatus
- `OPEN` - Route is fully operational
- `CLOSED` - Route is completely closed
- `RESTRICTED` - Route has restrictions or limited access
- `MAINTENANCE` - Route under maintenance with potential delays

### ChainControlStatus  
- `NONE` - No traction requirements
- `ADVISED` - Chains advised but not required
- `REQUIRED` - Chains or traction tires required
- `PROHIBITED` - Chains prohibited (typically urban areas)

### CongestionLevel
- `LIGHT` - Free-flowing traffic
- `MODERATE` - Some congestion, minor delays
- `HEAVY` - Significant congestion, major delays  
- `SEVERE` - Stop-and-go or stopped traffic

### AlertType
- `CLOSURE` - Road closure or blockage
- `CONSTRUCTION` - Construction zone or work area
- `INCIDENT` - Accident or emergency response
- `WEATHER` - Weather-related conditions

### AlertSeverity
- `INFO` - Informational, minor impact
- `WARNING` - Moderate impact, caution advised
- `CRITICAL` - Major impact, significant delays or safety concerns

### WeatherAlertType
- `WINTER_STORM` - Snow, ice, winter weather
- `FLOOD` - Flooding or water hazards
- `WIND` - High wind conditions
- `FOG` - Low visibility fog
- `HEAT` - Extreme heat advisories

## Relationships

```
Route 1:1 TrafficCondition
Route 1:N RouteAlert
Route 1:1 ChainControlStatus

WeatherData 1:N WeatherAlert
WeatherData 1:1 Coordinates

CacheEntry N:1 Source (routes or weather)
```

## State Transitions

### Route Status Transitions
```
OPEN ↔ RESTRICTED ↔ CLOSED
OPEN ↔ MAINTENANCE ↔ CLOSED
```

### Alert Lifecycle  
```
Created → Active → [Updated] → Resolved/Expired
```

### Cache Entry Lifecycle
```
Fresh → Stale → [Refreshed] → Fresh
Fresh → Stale → [Expired] → Removed
```

## Data Sources

- **Traffic Data**: Google Routes API (traffic conditions, speed readings, polylines)
- **Road Status**: Caltrans KML Feeds (road closures, chain control, CHP incidents)
  - Chain Controls: `https://quickmap.dot.ca.gov/data/cc.kml`
  - Lane Closures: `https://quickmap.dot.ca.gov/data/lcs2way.kml`
  - CHP Incidents: `https://quickmap.dot.ca.gov/data/chp-only.kml`
- **Weather Data**: OpenWeatherMap API (current conditions, weather alerts)
- **Configuration**: YAML config file + environment variables (route definitions with lat/lon)
- **Cache**: In-memory storage with configurable TTL per data source