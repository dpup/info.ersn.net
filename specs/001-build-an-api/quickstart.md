# Quickstart Guide - Live Data API Server

## Prerequisites

- Go 1.21+
- protoc (Protocol Buffer compiler)
- protoc-gen-go and protoc-gen-grpc-gateway plugins
- Google Routes API key
- OpenWeatherMap API key

## Installation

1. **Clone and setup project**:
   ```bash
   git clone <repository>
   cd server
   go mod init github.com/dpup/info.ersn.net/server
   ```

2. **Install dependencies**:
   ```bash
   go get github.com/dpup/prefab
   go get google.golang.org/grpc
   go get github.com/grpc-ecosystem/grpc-gateway/v2
   go get google.golang.org/protobuf
   go get github.com/stretchr/testify
   go get github.com/twpayne/go-kml
   ```

3. **Install protoc plugins**:
   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
   ```

## Configuration

1. **Create config.yaml**:
   ```yaml
   server:
     port: 8080
     cors_origins: ["*"]

   routes:
     refresh_interval: "5m"
     stale_threshold: "10m"
     google_api_key: "${GOOGLE_ROUTES_API_KEY}"
     monitored_routes:
       - name: "I-5 Seattle to Portland"
         id: "i5-sea-pdx"
         origin:
           latitude: 47.6062
           longitude: -122.3321
         destination:
           latitude: 45.5152
           longitude: -122.6784
       - name: "US-101 Coastal Route"  
         id: "us101-coastal"
         origin:
           latitude: 46.9741
           longitude: -124.1060  
         destination:
           latitude: 43.7749
           longitude: -124.5087

   weather:
     refresh_interval: "5m"
     stale_threshold: "10m"
     openweather_api_key: "${OPENWEATHER_API_KEY}"
     locations:
       - id: "seattle"
         name: "Seattle, WA"
         lat: 47.6062
         lon: -122.3321
       - id: "portland"  
         name: "Portland, OR"
         lat: 45.5152
         lon: -122.6784
   ```

2. **Set environment variables**:
   ```bash
   export GOOGLE_ROUTES_API_KEY="your-google-api-key"
   export OPENWEATHER_API_KEY="your-openweather-api-key"
   # Note: Caltrans KML feeds require no API keys
   ```

## Build and Run

1. **Build everything (protobuf generation + server + CLI tools)**:
   ```bash
   make build
   ```

2. **Test individual API clients (optional)**:
   ```bash
   # Test Google Routes API
   make test-google ROUTE_ID=i5-sea-pdx
   
   # Test Caltrans KML feeds
   make test-caltrans VERBOSE=true
   
   # Test OpenWeatherMap API
   make test-weather LOCATION_ID=seattle
   ```

3. **Run the server**:
   ```bash
   make run
   ```

   The server will start on port 8080 with both gRPC and REST endpoints available.

### Manual Build Steps (Alternative)

If you prefer manual commands:

1. **Generate protobuf code**:
   ```bash
   make proto
   # Or manually:
   # protoc --proto_path=api/v1 \
   #        --go_out=. --go_opt=paths=source_relative \
   #        --go-grpc_out=. --go-grpc_opt=paths=source_relative \
   #        --grpc-gateway_out=. --grpc-gateway_opt=paths=source_relative \
   #        api/v1/*.proto
   ```

2. **Build individual components**:
   ```bash
   make server        # Build main server only
   make tools         # Build CLI testing tools only
   make clean         # Clean build artifacts
   ```

## API Usage Examples

### REST API (via gRPC Gateway)

1. **Get all route conditions**:
   ```bash
   curl http://localhost:8080/api/v1/routes
   ```
   
   Expected response:
   ```json
   {
     "routes": [
       {
         "id": "i5-sea-pdx",
         "name": "I-5 Seattle to Portland",
         "status": "ROUTE_STATUS_OPEN",
         "traffic_condition": {
           "route_id": "i5-sea-pdx",
           "congestion_level": "CONGESTION_LEVEL_LIGHT",
           "average_speed_mph": 65,
           "typical_speed_mph": 65,
           "delays_minutes": 0
         },
         "chain_control": "CHAIN_CONTROL_NONE",
         "alerts": []
       }
     ],
     "last_updated": "2025-09-09T10:30:00Z"
   }
   ```

2. **Get specific route**:
   ```bash
   curl http://localhost:8080/api/v1/routes/i5-sea-pdx
   ```

3. **Get all weather data**:
   ```bash
   curl http://localhost:8080/api/v1/weather
   ```
   
   Expected response:
   ```json
   {
     "weather_data": [
       {
         "location_id": "seattle",
         "location_name": "Seattle, WA",
         "coordinates": {"latitude": 47.6062, "longitude": -122.3321},
         "current_condition": {
           "main": "Clear",
           "description": "clear sky",
           "humidity_percent": 65
         },
         "temperature_f": 72.5,
         "visibility_miles": 10,
         "wind_speed_mph": 5.2,
         "alerts": []
       }
     ],
     "last_updated": "2025-09-09T10:30:00Z"
   }
   ```

4. **Get weather alerts**:
   ```bash
   curl http://localhost:8080/api/v1/weather/alerts
   ```

### gRPC API

Use any gRPC client (like grpcui or grpcurl) to interact with the services:

```bash
# Install grpcurl for testing
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# List available services
grpcurl -plaintext localhost:8080 list

# Call GetRoutes
grpcurl -plaintext localhost:8080 api.v1.RoadsService/GetRoutes
```

## Testing the Integration

1. **Verify API responses**:
   - Routes endpoint returns configured routes
   - Weather endpoint returns configured locations
   - Data includes timestamps and proper structures

2. **Test caching behavior**:
   - Make multiple requests within 5 minutes
   - Verify `last_updated` timestamp remains the same
   - Wait 5+ minutes and verify timestamp updates

3. **Test CORS for static website**:
   ```bash
   curl -H "Origin: http://localhost:3000" \
        -H "Access-Control-Request-Method: GET" \
        -H "Access-Control-Request-Headers: X-Requested-With" \
        -X OPTIONS \
        http://localhost:8080/api/v1/routes
   ```

## Troubleshooting

- **Port conflicts**: Change port in config.yaml or set `PORT` environment variable
- **API key issues**: Verify environment variables are set correctly  
- **CORS issues**: Update `cors_origins` in config.yaml for your domain
- **Missing data**: Check API keys have proper permissions and quotas
- **Cache not refreshing**: Verify `refresh_interval` configuration

### Testing Individual Components

Use the CLI testing tools to isolate issues:

1. **Google Routes API issues**:
   ```bash
   make test-google ROUTE_ID=i5-sea-pdx VERBOSE=true
   ```
   - Validates API key and quota
   - Tests coordinate-based routing
   - Shows raw JSON response parsing

2. **Caltrans KML feed issues**:
   ```bash
   make test-caltrans VERBOSE=true FORMAT=table
   ```
   - Tests KML feed accessibility
   - Validates KML parsing logic
   - Shows geographic filtering results

3. **OpenWeatherMap API issues**:
   ```bash
   make test-weather LOCATION_ID=seattle VERBOSE=true
   ```
   - Validates weather API key
   - Tests location-based queries
   - Shows parsed weather data structure

### Development Workflow

1. **Clean builds**: `make clean && make build`
2. **Testing changes**: Use `make test-*` targets to verify parsing logic before running full server
3. **Configuration validation**: `make test-config` to validate configuration without API calls
4. **Full test suite**: `make test` to run all automated tests
5. **Docker deployment**: `make docker` to build container image

## Integration with Static Website

Add to your static website JavaScript:

```javascript
// Fetch route conditions
async function getRouteConditions() {
  const response = await fetch('http://localhost:8080/api/v1/routes');  
  const data = await response.json();
  return data.routes;
}

// Fetch weather data  
async function getWeatherData() {
  const response = await fetch('http://localhost:8080/api/v1/weather');
  const data = await response.json();
  return data.weather_data;
}

// Update sidebar with live data
async function updateSidebar() {
  try {
    const routes = await getRouteConditions();
    const weather = await getWeatherData();
    
    // Update your sidebar DOM elements
    updateRouteDisplay(routes);
    updateWeatherDisplay(weather);
  } catch (error) {
    console.error('Failed to fetch live data:', error);
  }
}

// Refresh every 30 seconds
setInterval(updateSidebar, 30000);
updateSidebar(); // Initial load
```

## Makefile Targets

Common development tasks are automated via the Makefile:

### Build Targets
- `make build` - Build server and all CLI tools
- `make server` - Build main server only
- `make tools` - Build CLI testing tools only
- `make proto` - Generate protobuf code
- `make clean` - Clean build artifacts

### Testing Targets
- `make test` - Run full test suite
- `make test-google [ROUTE_ID=id] [VERBOSE=true]` - Test Google Routes API
- `make test-caltrans [VERBOSE=true] [FORMAT=table]` - Test Caltrans KML feeds
- `make test-weather [LOCATION_ID=id] [VERBOSE=true]` - Test OpenWeatherMap API
- `make test-config` - Validate configuration without API calls

### Development Targets
- `make run [CONFIG=config.yaml]` - Run server with configuration
- `make dev` - Run server in development mode with auto-restart
- `make lint` - Run Go linting tools
- `make fmt` - Format Go code

### Deployment Targets
- `make docker` - Build Docker container image
- `make deploy` - Deploy to configured environment
- `make install` - Install CLI tools to system PATH

## Next Steps

1. Implement service logic in `internal/services/`
2. Add external API clients in `internal/clients/`  
3. Set up comprehensive testing suite using `make test`
4. Add monitoring and logging
5. Deploy to production environment using `make deploy`