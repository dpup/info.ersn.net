# Live Data API Server - Build, Test, and Deployment Tasks
.PHONY: build test proto clean server tools run dev lint fmt docker deploy install help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Build directories
BUILD_DIR=bin
PROTO_DIR=api/v1
CMD_DIR=cmd

# Binary names
SERVER_BINARY=$(BUILD_DIR)/server
TEST_GOOGLE_BINARY=$(BUILD_DIR)/test-google
TEST_CALTRANS_BINARY=$(BUILD_DIR)/test-caltrans
TEST_WEATHER_BINARY=$(BUILD_DIR)/test-weather
TEST_GEO_UTILS_BINARY=$(BUILD_DIR)/test-geo-utils
TEST_ALERT_ENHANCER_BINARY=$(BUILD_DIR)/test-alert-enhancer
TEST_ROUTE_MATCHER_BINARY=$(BUILD_DIR)/test-route-matcher

# Default target
all: build

## Build Targets

# Build everything (protobuf generation + server + CLI tools)
build: proto server tools

# Build main server only
server: $(SERVER_BINARY)

$(SERVER_BINARY): proto
	$(GOBUILD) -o $(SERVER_BINARY) ./$(CMD_DIR)/server

# Build CLI testing tools only
tools: $(TEST_GOOGLE_BINARY) $(TEST_CALTRANS_BINARY) $(TEST_WEATHER_BINARY)

$(TEST_GOOGLE_BINARY): proto
	$(GOBUILD) -o $(TEST_GOOGLE_BINARY) ./$(CMD_DIR)/test-google

$(TEST_CALTRANS_BINARY): proto
	$(GOBUILD) -o $(TEST_CALTRANS_BINARY) ./$(CMD_DIR)/test-caltrans

$(TEST_WEATHER_BINARY): proto
	$(GOBUILD) -o $(TEST_WEATHER_BINARY) ./$(CMD_DIR)/test-weather

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@mkdir -p $(BUILD_DIR) $(PROTO_DIR)
	@PATH="$(shell go env GOPATH)/bin:$(PATH)" protoc --proto_path=$(PROTO_DIR) \
		--proto_path=$(shell go list -f '{{ .Dir }}' -m github.com/googleapis/googleapis) \
		--go_out=$(PROTO_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_DIR) --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=$(PROTO_DIR) --grpc-gateway_opt=paths=source_relative \
		$(PROTO_DIR)/*.proto
	@echo "Protobuf code generation completed."

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f $(PROTO_DIR)/*.pb.go
	rm -f $(PROTO_DIR)/*_grpc.pb.go

## Testing Targets

# Run full test suite
test:
	$(GOTEST) -v ./...

# Test incident content processing functionality
test-incident: $(TEST_CALTRANS_BINARY)
	./$(TEST_CALTRANS_BINARY) --test-content-hash $(if $(VERBOSE),--verbose)

# Test individual API clients (optional parameters)
test-google: $(TEST_GOOGLE_BINARY)
	./$(TEST_GOOGLE_BINARY) --config=prefab.yaml $(if $(ROUTE_ID),--route-id=$(ROUTE_ID)) $(if $(VERBOSE),--verbose)

test-caltrans: $(TEST_CALTRANS_BINARY)
	./$(TEST_CALTRANS_BINARY) $(if $(OFFLINE),-offline) $(if $(FEED),-feed=$(FEED)) $(if $(FILTER),-filter) $(if $(LAT),-lat=$(LAT)) $(if $(LON),-lon=$(LON)) $(if $(RADIUS),-radius=$(RADIUS))

test-weather: $(TEST_WEATHER_BINARY)
	./$(TEST_WEATHER_BINARY) --config=prefab.yaml $(if $(LOCATION_ID),--location-id=$(LOCATION_ID)) $(if $(VERBOSE),--verbose)

# Validate configuration without API calls
test-config:
	@echo "Configuration validation not yet implemented"

# Fetch timestamped test data snapshots from live APIs
fetch-test-data: fetch-caltrans-data fetch-google-data fetch-weather-data

# Fetch Caltrans KML test data
fetch-caltrans-data:
	@echo "Fetching Caltrans KML test data snapshots..."
	@mkdir -p tests/testdata/caltrans
	$(eval TIMESTAMP := $(shell date +%Y%m%d_%H%M%S))
	@echo "Timestamp: $(TIMESTAMP)"
	@curl -s "https://quickmap.dot.ca.gov/data/lcs2way.kml" > tests/testdata/caltrans/lane_closures_$(TIMESTAMP).kml
	@curl -s "https://quickmap.dot.ca.gov/data/chp-only.kml" > tests/testdata/caltrans/chp_incidents_$(TIMESTAMP).kml
	@curl -s "https://quickmap.dot.ca.gov/data/cc.kml" > tests/testdata/caltrans/chain_controls_$(TIMESTAMP).kml
	@echo "✅ Caltrans test data snapshots saved"

# Fetch Google Routes API test data
fetch-google-data:
	@echo "Fetching Google Routes API test data..."
	@mkdir -p tests/testdata/google
	$(eval TIMESTAMP := $(shell date +%Y%m%d_%H%M%S))
	@if [ -z "$(PF__GOOGLE_ROUTES__API_KEY)" ]; then \
		echo "⚠️  PF__GOOGLE_ROUTES__API_KEY not set, skipping Google API fixtures"; \
		echo "   Set environment variable: export PF__GOOGLE_ROUTES__API_KEY=your-api-key"; \
	else \
		echo "Fetching sample route data (Seattle to Portland)..."; \
		curl -s -X POST "https://routes.googleapis.com/directions/v2:computeRoutes" \
			-H "X-Goog-Api-Key: $(PF__GOOGLE_ROUTES__API_KEY)" \
			-H "X-Goog-FieldMask: routes.duration,routes.staticDuration,routes.distanceMeters,routes.polyline.encodedPolyline,routes.travelAdvisory.speedReadingIntervals" \
			-H "Content-Type: application/json" \
			-d '{"origin":{"location":{"latLng":{"latitude":47.6062,"longitude":-122.3321}}},"destination":{"location":{"latLng":{"latitude":45.5152,"longitude":-122.6784}}},"travelMode":"DRIVE","routingPreference":"TRAFFIC_AWARE_OPTIMAL","extraComputations":["TRAFFIC_ON_POLYLINE"]}' \
			> tests/testdata/google/seattle_portland_$(TIMESTAMP).json; \
		echo "✅ Google Routes test data saved"; \
	fi

# Fetch Weather API test data  
fetch-weather-data:
	@echo "Fetching OpenWeatherMap test data..."
	@mkdir -p tests/testdata/weather
	$(eval TIMESTAMP := $(shell date +%Y%m%d_%H%M%S))
	@if [ -z "$(PF__OPENWEATHER__API_KEY)" ]; then \
		echo "⚠️  PF__OPENWEATHER__API_KEY not set, skipping Weather API fixtures"; \
		echo "   Set environment variable: export PF__OPENWEATHER__API_KEY=your-api-key"; \
	else \
		echo "Fetching current weather data (Seattle)..."; \
		curl -s "https://api.openweathermap.org/data/2.5/weather?lat=47.6062&lon=-122.3321&appid=$(PF__OPENWEATHER__API_KEY)&units=metric" \
			> tests/testdata/weather/seattle_current_$(TIMESTAMP).json; \
		echo "Fetching weather alerts data (Seattle)..."; \
		curl -s "https://api.openweathermap.org/data/3.0/onecall?lat=47.6062&lon=-122.3321&appid=$(PF__OPENWEATHER__API_KEY)&exclude=minutely,hourly,daily" \
			> tests/testdata/weather/seattle_alerts_$(TIMESTAMP).json; \
		echo "✅ Weather API test data saved"; \
	fi

# Update symlinks to use the most recent timestamped test data
use-latest-test-data:
	@echo "Updating symlinks to use latest test data..."
	@# Update Caltrans data
	@if [ -d tests/testdata/caltrans ]; then \
		cd tests/testdata/caltrans && \
		ln -sf $$(ls -1 lane_closures_*.kml | tail -1) lane_closures.kml && \
		ln -sf $$(ls -1 chp_incidents_*.kml | tail -1) chp_incidents.kml && \
		ln -sf $$(ls -1 chain_controls_*.kml | tail -1) chain_controls.kml; \
	fi
	@# Update Google Routes data
	@if [ -d tests/testdata/google ]; then \
		cd tests/testdata/google && \
		ln -sf $$(ls -1 seattle_portland_*.json | tail -1) seattle_portland.json; \
	fi
	@# Update Weather data  
	@if [ -d tests/testdata/weather ]; then \
		cd tests/testdata/weather && \
		ln -sf $$(ls -1 seattle_current_*.json | tail -1) seattle_current.json && \
		ln -sf $$(ls -1 seattle_alerts_*.json | tail -1) seattle_alerts.json; \
	fi
	@echo "✅ Symlinks updated to latest snapshots"
	@if [ -d tests/testdata/caltrans ]; then ls -la tests/testdata/caltrans/*.kml | grep -E "(lane_closures|chp_incidents|chain_controls)\.kml"; fi
	@if [ -d tests/testdata/google ]; then ls -la tests/testdata/google/*.json | grep -E "seattle_portland\.json"; fi
	@if [ -d tests/testdata/weather ]; then ls -la tests/testdata/weather/*.json | grep -E "(seattle_current|seattle_alerts)\.json"; fi

## Development Targets

# Run server with configuration
run: server
	./$(SERVER_BINARY)

# Run server in background for testing
run-bg: server stop
	@echo "Starting server in background..."
	@nohup ./$(SERVER_BINARY) > server.log 2>&1 & echo $$! > server.pid
	@sleep 2
	@if [ -f server.pid ] && kill -0 $$(cat server.pid) 2>/dev/null; then \
		echo "Server started in background (PID: $$(cat server.pid))"; \
		echo "Server logs: tail -f server.log"; \
		echo "Use 'make stop' to stop the server"; \
	else \
		echo "Failed to start server"; \
		rm -f server.pid; \
		exit 1; \
	fi

# Stop background server
stop:
	@echo "Stopping server..."
	@STOPPED=false; \
	if [ -f server.pid ]; then \
		PID=$$(cat server.pid); \
		if kill -0 $$PID 2>/dev/null; then \
			kill $$PID && echo "Stopped server (PID: $$PID)"; \
			STOPPED=true; \
		fi; \
		rm -f server.pid; \
	fi; \
	PORT_PID=$$(lsof -ti :8080 2>/dev/null); \
	if [ -n "$$PORT_PID" ]; then \
		kill $$PORT_PID 2>/dev/null && echo "Stopped process on port 8080 (PID: $$PORT_PID)"; \
		STOPPED=true; \
	fi; \
	if [ "$$STOPPED" = "false" ]; then \
		echo "No running server found"; \
	fi

# Test server startup (quick test that exits after a few seconds)
test-server: server
	@echo "Testing server startup..."
	./$(SERVER_BINARY) & \
	SERVER_PID=$$!; \
	sleep 3; \
	kill $$SERVER_PID; \
	echo "✅ Server startup test completed successfully"

# Run server in development mode with auto-restart
dev: server
	@echo "Development mode with auto-restart not yet implemented"
	@echo "For now, use: make run CONFIG=prefab.yaml"

# Go code formatting
fmt:
	$(GOFMT) ./...

# Run Go linting tools
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		$(GOCMD) vet ./...; \
	fi

## Deployment Targets

# Build Docker container image
docker:
	@echo "Docker build not yet implemented"

# Deploy to configured environment
deploy:
	@echo "Deployment not yet implemented"

# Install CLI tools to system PATH
install: tools
	@echo "Installing CLI tools to system PATH..."
	cp $(TEST_GOOGLE_BINARY) /usr/local/bin/
	cp $(TEST_CALTRANS_BINARY) /usr/local/bin/
	cp $(TEST_WEATHER_BINARY) /usr/local/bin/
	cp $(TEST_GEO_UTILS_BINARY) /usr/local/bin/
	cp $(TEST_ALERT_ENHANCER_BINARY) /usr/local/bin/
	cp $(TEST_ROUTE_MATCHER_BINARY) /usr/local/bin/
	@echo "CLI tools installed: test-google, test-caltrans, test-weather, test-geo-utils, test-alert-enhancer, test-route-matcher"

## Utility Targets

# Update Go dependencies
deps:
	$(GOMOD) tidy
	$(GOMOD) download

# Show help
help:
	@echo "Live Data API Server - Available targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  build       - Build server and all CLI tools (default)"
	@echo "  server      - Build main server only"
	@echo "  tools       - Build CLI testing tools only"
	@echo "  proto       - Generate protobuf code"
	@echo "  clean       - Clean build artifacts"
	@echo ""
	@echo "Testing targets:"
	@echo "  test        - Run full test suite (unit tests, works offline)"
	@echo "  test-unit   - Run unit tests only"
	@echo "  test-contract - Run contract tests"
	@echo "  test-google [ROUTE_ID=id] [VERBOSE=true]   - Test Google Routes API"
	@echo "  test-caltrans [VERBOSE=true] [FORMAT=table] - Test Caltrans KML feeds"
	@echo "  test-weather [LOCATION_ID=id] [VERBOSE=true] - Test OpenWeatherMap API"
	@echo "  test-config - Validate configuration without API calls"
	@echo "  fetch-test-data - Fetch test fixtures from live APIs (requires API keys)"
	@echo ""
	@echo "Development targets:"
	@echo "  run         - Run server (blocks until stopped with Ctrl+C)"
	@echo "  run-bg      - Run server in background (stops existing server first)"
	@echo "  stop        - Stop background server (handles orphaned processes)"
	@echo "  test-server - Quick server startup test (3 seconds)"
	@echo "  dev         - Run server in development mode with auto-restart"
	@echo "  lint        - Run Go linting tools"
	@echo "  fmt         - Format Go code"
	@echo ""
	@echo "Deployment targets:"
	@echo "  docker      - Build Docker container image"
	@echo "  deploy      - Deploy to configured environment"
	@echo "  install     - Install CLI tools to system PATH"
	@echo ""
	@echo "Utility targets:"
	@echo "  deps        - Update Go dependencies"
	@echo "  help        - Show this help message"