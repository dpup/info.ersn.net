# ERSN Info Server

A real-time API server providing road conditions and weather information for the Ebbett's Pass region, combining data from Google Routes API, Caltrans feeds, and OpenWeatherMap.

**Live API available at: https://info.ersn.net**

## Features

- **Real-time Road Conditions**: Live traffic data, travel times, and congestion levels
- **Weather Information**: Current conditions and alerts for multiple locations
- **Smart Alert Filtering**: Route-aware classification (ON_ROUTE/NEARBY/DISTANT) with distance-based relevance
- **AI-Enhanced Descriptions**: Optional OpenAI integration to convert technical alerts into human-readable summaries
- **Road Alerts**: Chain control requirements, lane closures, and CHP incidents with polyline geometry
- **REST API**: HTTP endpoints with JSON responses
- **gRPC Support**: Native gRPC services with HTTP gateway
- **Caching**: In-memory caching with configurable refresh intervals

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
      "alerts": []
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
    "alerts": []
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
   export PF__ROADS__GOOGLE_ROUTES__API_KEY="your-google-api-key"
   export PF__WEATHER__OPENWEATHER_API_KEY="your-openweather-api-key"
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

The server uses `prefab.yaml` for configuration and supports environment variable overrides using the `PF__` prefix:

```yaml
server:
  port: 8080

roads:
  google_routes:
    api_key: ""  # Set via PF__ROADS__GOOGLE_ROUTES__API_KEY
    refresh_interval: "5m"
    stale_threshold: "10m"
  monitored_roads:
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
  openweather_api_key: ""  # Set via PF__WEATHER__OPENWEATHER_API_KEY
  refresh_interval: "5m"
  stale_threshold: "10m"
  locations:
    - id: "murphys"
      name: "Murphys, CA"
      lat: 38.139117
      lon: -120.456111
```

#### AI-Enhanced Alerts (Optional)

The server supports AI-powered alert enhancement using OpenAI's GPT models to transform technical Caltrans descriptions into human-readable summaries:

```yaml
roads:
  openai:
    enabled: false             # Set to true to enable AI enhancement
    api_key: ""                # Set via PF__ROADS__OPENAI__API_KEY
    model: "gpt-3.5-turbo"     # OpenAI model for enhancement
    timeout: "30s"             # API timeout
    max_retries: 3             # Retry attempts
```

**To enable AI enhancement:**

1. Get an OpenAI API key from https://platform.openai.com/api-keys
2. Set the environment variable:
   ```bash
   export PF__ROADS__OPENAI__API_KEY="sk-your-openai-api-key"
   ```
3. Enable in configuration:
   ```bash
   # Update prefab.yaml or set via environment
   export PF__ROADS__OPENAI__ENABLED=true
   ```
4. Restart the server

**Benefits of AI enhancement:**
- Converts technical jargon like "Rte 4 EB of MM 31 - VEHICLE IN DITCH, EMS ENRT" 
- Into clear descriptions like "Vehicle accident in ditch on Highway 4 eastbound near mile marker 31, emergency services en route"
- Provides structured data with impact assessment and duration estimates
- Generates condensed summaries for mobile displays

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
├── tests/                     # Test files
│   ├── contract/              # gRPC contract tests
│   ├── integration/           # External API integration tests
│   └── unit/                  # Unit tests
├── prefab.yaml               # Server configuration
└── Makefile                  # Build automation
```

### Adding New Roads

1. Update `prefab.yaml` with new road coordinates:
   ```yaml
   monitored_roads:
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
   ./bin/test-google
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