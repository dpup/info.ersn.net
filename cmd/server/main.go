package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/dpup/prefab"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/cache"
	"github.com/dpup/info.ersn.net/server/internal/clients/caltrans"
	"github.com/dpup/info.ersn.net/server/internal/clients/google"
	"github.com/dpup/info.ersn.net/server/internal/clients/weather"
	"github.com/dpup/info.ersn.net/server/internal/config"
	"github.com/dpup/info.ersn.net/server/internal/lib/alerts"
	"github.com/dpup/info.ersn.net/server/internal/services"
)

func main() {
	// Load configuration using Prefab's config system
	appConfig := loadConfig()

	// Initialize cache
	cacheInstance := cache.NewCache()

	// Initialize external API clients
	googleClient := google.NewClient(appConfig.Roads.GoogleRoutes.APIKey)
	caltransClient := caltrans.NewFeedParser()
	weatherClient := weather.NewClient(appConfig.Weather.OpenWeatherAPIKey)

	// Initialize OpenAI enhancer with caching (required for service)
	if appConfig.Roads.OpenAI.APIKey == "" {
		log.Fatal("OpenAI API key is required in configuration for incident enhancement")
	}

	model := appConfig.Roads.OpenAI.Model

	// Create OpenAI enhancer (caching is now integrated directly in RoadsService)
	alertEnhancer := alerts.NewAlertEnhancer(appConfig.Roads.OpenAI.APIKey, model)
	
	log.Printf("OpenAI enhancement enabled with integrated content-based caching (model: %s)", model)

	// Initialize gRPC services  
	roadsService := services.NewRoadsService(googleClient, caltransClient, cacheInstance, &appConfig.Roads, alertEnhancer)
	weatherService := services.NewWeatherService(weatherClient, cacheInstance, &appConfig.Weather)

	log.Printf("Live Data API Server starting")
	log.Printf("Roads monitored: %d", len(appConfig.Roads.MonitoredRoads))
	log.Printf("Weather locations: %d", len(appConfig.Weather.Locations))

	// Start periodic refresh to maintain cache warmth (replaces complex cache warmer)
	periodicRefresh := services.NewPeriodicRefreshService(roadsService, &appConfig.Roads)
	if err := periodicRefresh.StartPeriodicRefresh(context.Background()); err != nil {
		log.Printf("Failed to start periodic refresh: %v", err)
	}

	// Create Prefab server with GRPC reflection enabled
	// Server configuration (port, etc.) will be loaded from prefab.yaml/env vars
	server := prefab.New(
		prefab.WithGRPCReflection(),
		prefab.WithHTTPHandlerFunc("/", homepageHandler),
	)

	// Register gRPC services using Prefab's service registrar
	api.RegisterRoadsServiceServer(server.ServiceRegistrar(), roadsService)
	api.RegisterWeatherServiceServer(server.ServiceRegistrar(), weatherService)

	// Register gateway handlers using Prefab's gateway args
	if err := api.RegisterRoadsServiceHandlerFromEndpoint(server.GatewayArgs()); err != nil {
		log.Fatalf("Failed to register Roads service gateway: %v", err)
	}

	if err := api.RegisterWeatherServiceHandlerFromEndpoint(server.GatewayArgs()); err != nil {
		log.Fatalf("Failed to register Weather service gateway: %v", err)
	}

	// Start the server (blocks until shutdown)
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// loadConfig loads configuration using Prefab's config system
// Configuration is loaded from prefab.yaml and environment variables with PF__ prefix
func loadConfig() *config.Config {
	appConfig := &config.Config{}

	// Unmarshal specific sections from Prefab's config using exact key paths
	if err := prefab.Config.Unmarshal("roads", &appConfig.Roads); err != nil {
		log.Fatalf("Failed to unmarshal roads section: %v", err)
	}

	if err := prefab.Config.Unmarshal("weather", &appConfig.Weather); err != nil {
		log.Fatalf("Failed to unmarshal weather section: %v", err)
	}

	return appConfig
}

// homepageHandler serves a simple HTML homepage at the server root
func homepageHandler(w http.ResponseWriter, r *http.Request) {
	// Only handle the root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>info.ersn.net</title>
    <style>
        body { 
            font-family: 'Courier New', Consolas, monospace; 
            background: #000; 
            color: #0f0; 
            padding: 20px; 
            line-height: 1.4; 
        }
        a { color: #0ff; text-decoration: none; }
        a:hover { text-decoration: underline; }
        pre { margin: 0; }
        .header { color: #ff0; }
        .section { margin: 20px 0; }
    </style>
</head>
<body>
<pre>
<span class="header">info.ersn.net</span>

Real-time API server providing road conditions and weather information 
for the Ebbett's Pass region.

<span class="header">Repository:</span>
<a href="https://github.com/dpup/info.ersn.net">https://github.com/dpup/info.ersn.net</a>

<span class="header">API Endpoints:</span>

Roads API:
  <a href="/api/v1/roads">GET /api/v1/roads</a>                 - List all monitored roads
  <a href="/api/v1/roads/hwy4-angels-murphys">GET /api/v1/roads/{road_id}</a>       - Get specific road details

Weather API:
  <a href="/api/v1/weather">GET /api/v1/weather</a>               - Current weather for all locations
  <a href="/api/v1/weather/alerts">GET /api/v1/weather/alerts</a>        - Active weather alerts

<span class="header">Data Sources:</span>
  • Google Routes API    - Traffic conditions and travel times
  • OpenWeatherMap API   - Weather data and alerts  
  • Caltrans KML Feeds   - Lane closures and CHP incidents

<span class="header">Example Usage:</span>
  curl <a href="/api/v1/roads">https://info.ersn.net/api/v1/roads</a>
  curl <a href="/api/v1/weather">https://info.ersn.net/api/v1/weather</a>
</pre>
</body>
</html>`

	if _, err := fmt.Fprint(w, html); err != nil {
		slog.Error("Failed to write homepage HTML", "error", err)
	}
}
