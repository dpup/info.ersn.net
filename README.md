# ERSN Info Server

A real-time API server providing road conditions and weather information for the Ebbett's Pass region, combining data from Google Routes API, Caltrans feeds, and OpenWeatherMap.

## Overview

The server dynamically builds routes between geographic points using the Google Routes API, retrieving real-time traffic data and estimated travel times. Polyline geometry from Google is used to cross-reference Caltrans feeds, with alerts filtered and classified as on-route or nearby based on spatial relevance. To improve usability, OpenAI is integrated to automatically convert technical Caltrans alerts into clear, human-readable summaries.

Weather data is independently sourced from OpenWeatherMap for each configured location, providing current conditions and active alerts.

The architecture is modular and location-agnostic, allowing easy adaptation to other regions or road networks by updating configuration.

**Live API available at: https://info.ersn.net**

## Features

- **Real-time Road Conditions**: Live traffic data, travel times, and congestion levels from Google Routes API
- **Intelligent Road Alerts**: AI-enhanced alerts from Caltrans feeds including lane closures, CHP incidents, and construction
- **Smart Alert Classification**: Route-aware filtering (ON_ROUTE/NEARBY/DISTANT) based on spatial analysis
- **AI-Enhanced Descriptions**: Automatic OpenAI conversion of technical alerts into clear, human-readable summaries
- **Weather Information**: Current conditions and alerts for multiple locations from OpenWeatherMap
- **Stale-while-revalidate Caching**: Sub-100ms responses by serving cached data while refreshing in background
- **REST API**: Clean HTTP endpoints with comprehensive JSON responses
- **gRPC Support**: Native gRPC services with automatic HTTP gateway
- **Configurable & Scalable**: Location-agnostic architecture with simple YAML configuration

## API Endpoints

### Roads API

#### List All Roads
```http
GET /api/v1/roads
```

**Response Example:**
```json
{
  "roads": [
    {
      "id": "hwy4-angels-murphys",
      "name": "Hwy 4",
      "section": "Angels Camp to Murphys",
      "status": "OPEN",
      "durationMinutes": 11,
      "distanceKm": 13,
      "congestionLevel": "CLEAR",
      "delayMinutes": 0,
      "alerts": [
        {
          "type": "CONSTRUCTION",
          "severity": "INFO",
          "classification": "NEARBY",
          "title": "Lane Closure Advisory",
          "description": "Routine maintenance work on shoulder, no traffic impact expected.",
          "condensedSummary": "Shoulder work, no delays",
          "impact": "none",
        }
      ]
    }
  ],
  "lastUpdated": "2025-09-11T01:52:05.646618Z"
}
```

#### Get Specific Road
```http
GET /api/v1/roads/{road_id}
```

**Response Example:**
```json
{
  "road": {
    "id": "hwy4-murphys-arnold",
    "name": "Hwy 4",
    "section": "Murphys to Arnold",
    "status": "OPEN",
    "durationMinutes": 15,
    "distanceKm": 20,
    "congestionLevel": "CLEAR",
    "delayMinutes": 0,
    "alerts": [
      {
        "type": "INCIDENT",
        "severity": "WARNING",
        "classification": "ON_ROUTE",
        "title": "CHP Incident 250911GG0206",
        "description": "Multi-vehicle accident blocking right lane near mile marker 45. Expect 15-20 minute delays while emergency crews clear the scene.",
        "condensedSummary": "Accident blocking right lane, 15-20 min delays",
        "startTime": "2025-09-11T01:30:00Z",
        "location": {
          "latitude": 38.2345,
          "longitude": -120.1234
        },
        "locationDescription": "Highway 4 near mile marker 45",
        "impact": "moderate",
        "timeReported": "2025-09-11T01:30:00Z",
        "lastUpdated": "2025-09-11T01:45:00Z",
        "metadata": {
          "lanes_affected": "1 of 2",
          "emergency_services": "CHP on scene"
        }
      }
    ]
  },
  "lastUpdated": "2025-09-11T01:52:05.646618Z"
}
```

**Road Status Values:**
- `OPEN` - Road is open to traffic
- `CLOSED` - Road is closed
- `RESTRICTED` - Limited access or restrictions
- `MAINTENANCE` - Under maintenance

**Congestion Levels:**
- `CLEAR` - Free flowing traffic
- `LIGHT` - Light traffic
- `MODERATE` - Moderate traffic
- `HEAVY` - Heavy traffic
- `SEVERE` - Severe congestion

### Road Alerts

The Roads API provides intelligent alerts that combine data from multiple sources and uses AI enhancement for better readability.

**Alert Types:**
- `CLOSURE` - Road closures and lane restrictions
- `CONSTRUCTION` - Planned construction activities
- `INCIDENT` - Traffic accidents and emergency situations
- `WEATHER` - Weather-related road conditions

**Alert Severity:**
- `INFO` - Informational notices
- `WARNING` - Moderate impact on travel
- `CRITICAL` - Severe impact or safety concerns

**Alert Classification:**
- `ON_ROUTE` - Directly affects route path (< 100m from route)
- `NEARBY` - In surrounding area but not blocking route
- `DISTANT` - Too far from route to be relevant

**AI Enhancement Features:**
- **Smart Descriptions**: Technical Caltrans alerts are automatically converted to human-readable summaries using OpenAI
- **Impact Assessment**: AI evaluates impact levels: `none`, `light`, `moderate`, `severe`
- **Duration Estimates**: AI provides duration estimates: `unknown`, `< 1 hour`, `several hours`, `ongoing`
- **Condensed Summaries**: Short format optimized for mobile displays
- **Structured Metadata**: Additional contextual information like lanes affected, emergency services on scene

**Data Sources:**
- **Caltrans KML Feeds**: Lane closures, CHP incidents, and chain control status
- **Google Routes API**: Real-time traffic conditions and route geometry for spatial matching
- **OpenAI Enhancement**: Automatic conversion of technical alerts into clear, actionable information

*Note: Chain control status is currently disabled until winter when actual chain requirement data becomes available in Caltrans feeds. All roads will show no chain requirements.*

### Weather API

#### List All Weather Locations
```http
GET /api/v1/weather
```

**Response Example:**
```json
{
  "weatherData": [
    {
      "locationId": "murphys",
      "locationName": "Murphys, CA",
      "weatherMain": "Clear",
      "weatherDescription": "clear sky",
      "weatherIcon": "01d",
      "temperatureCelsius": 22,
      "feelsLikeCelsius": 21,
      "humidityPercent": 45,
      "windSpeedKmh": 8,
      "windDirectionDegrees": 230,
      "visibilityKm": 16,
      "alerts": []
    }
  ],
  "lastUpdated": "2025-09-11T01:52:05.646618Z"
}
```

#### Get Weather Alerts
```http
GET /api/v1/weather/alerts
```

**Response Example:**
```json
{
  "alerts": [
    {
      "id": "alert-123",
      "type": "WEATHER",
      "severity": "WARNING",
      "title": "Winter Weather Advisory",
      "description": "Snow expected above 7000 feet",
      "startTime": "2025-09-11T06:00:00Z",
      "endTime": "2025-09-11T18:00:00Z",
      "affectedAreas": ["Sierra Nevada Mountains"]
    }
  ],
  "lastUpdated": "2025-09-11T01:52:05.646618Z"
}
```

## Quick Start

### Prerequisites

- Go 1.21+
- Google Routes API key
- OpenWeatherMap API key

### Installation

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd info.ersn.net
   ```

2. Set up environment variables:
   ```bash
   export PF__GOOGLE_ROUTES__API_KEY="your-google-api-key"
   export PF__OPENWEATHER__API_KEY="your-openweather-api-key"
   export PF__OPENAI__API_KEY="your-openai-api-key"  # Optional, for AI-enhanced alerts
   ```

3. Build and run:
   ```bash
   make build
   make run
   ```

4. Test the API:
   ```bash
   # Test locally
   curl http://localhost:8080/api/v1/roads
   curl http://localhost:8080/api/v1/weather
   
   # Or test the live API
   curl https://info.ersn.net/api/v1/roads
   curl https://info.ersn.net/api/v1/weather
   ```

### Configuration

The server uses `prefab.yaml` for configuration with a **simplified structure** and supports environment variable overrides using the `PF__` prefix. Configuration has been streamlined with top-level client configs and consistent camelCase naming:

```yaml
# Client Configurations - Top Level
googleRoutes:
  # apiKey set via PF__GOOGLE_ROUTES__API_KEY

openai:
  # apiKey set via PF__OPENAI__API_KEY  
  model: "gpt-4o-mini"
  timeout: "30s"
  maxRetries: 3

openweather:
  # apiKey set via PF__OPENWEATHER__API_KEY

# Service Configurations
roads:
  refreshInterval: "5m"
  staleThreshold: "10m"
  monitoredRoads:
    - name: "Hwy 4"
      section: "Angels Camp to Murphys"
      id: "hwy4-angels-murphys"
      origin:
        latitude: 38.067400
        longitude: -120.540200
      destination:
        latitude: 38.139117
        longitude: -120.456111

weather:
  refreshInterval: "5m"
  staleThreshold: "10m"
  locations:
    - id: "murphys"
      name: "Murphys, CA"
      coordinates:
        latitude: 38.139117
        longitude: -120.456111
```

## Development

### Build Commands

```bash
# Build all binaries
make build

# Build specific components
make server    # Main API server
make tools     # CLI testing tools

# Generate protobuf code
make proto

# Clean build artifacts
make clean
```

### Running the Server

```bash
# Run in foreground
make run

# Run in background for testing
make run-bg

# Stop background server
make stop
```

### Testing

```bash
# Run all tests
make test

# Run specific test suites
make test-contract      # gRPC contract tests
make test-integration   # External API integration tests
make test-unit         # Unit tests

# Test individual API clients
make test-google       # Test Google Routes API
make test-caltrans     # Test Caltrans KML parsing
make test-weather      # Test OpenWeatherMap API
```

### Code Quality

```bash
# Run linting
make lint

# Format code
make fmt

# Run both linting and tests
make lint && make test
```

### Project Structure

```
/
├── api/v1/                     # Protocol Buffer definitions
│   ├── roads.proto            # gRPC service for road conditions
│   ├── weather.proto          # gRPC service for weather data
│   └── common.proto           # Shared proto definitions
├── cmd/                       # CLI applications
│   ├── server/                # Main API server
│   ├── test-google/           # Google Routes API testing tool
│   ├── test-caltrans/         # Caltrans data testing tool
│   └── test-weather/          # Weather API testing tool
├── internal/                  # Private application code
│   ├── services/              # gRPC service implementations
│   ├── clients/               # External API clients
│   ├── cache/                 # In-memory caching with TTL
│   └── config/                # Configuration management
├── tests/                     # Test support
│   └── testdata/              # Static fixture data
├── prefab.yaml                # Server configuration
├── Dockerfile                 # Sample docker file
└── Makefile                   # Build automation
```

### Adding New Roads

1. Update `prefab.yaml` with new road coordinates (note the simplified structure):
   ```yaml
   roads:
     monitoredRoads:  # Note: camelCase naming
       - name: "Highway Name"
         section: "Start to End"
         id: "unique-road-id"
         origin:
           latitude: 0.0
           longitude: 0.0
         destination:
           latitude: 0.0
           longitude: 0.0
   ```

2. Test with the Google Routes API tool:
   ```bash
  make test-google
   ```

3. Restart the server to pick up configuration changes:
   ```bash
   make stop && make run-bg
   ```

### API Rate Limits

- **Google Routes API**: 3,000 queries per minute
- **OpenWeatherMap API**: 60 calls per minute (free tier)
- **Caltrans KML Feeds**: No official limits, but feeds are refreshed every 5-30 minutes

### Architecture

The server uses a hybrid architecture:

- **gRPC Services**: Core business logic implemented as gRPC services
- **HTTP Gateway**: Automatic REST API generation from gRPC definitions
- **External Clients**: Dedicated clients for each external API
- **Caching Layer**: In-memory cache with TTL for performance
- **Configuration**: Prefab framework for flexible configuration management

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes following the existing code style
4. Run tests and linting (`make lint && make test`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## Support

For questions, issues, or feature requests, please open an issue on the GitHub repository.

## License

MIT License

Copyright (c) 2025 Daniel Pupius

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.