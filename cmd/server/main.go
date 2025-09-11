package main

import (
	"log"

	"github.com/dpup/prefab"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/cache"
	"github.com/dpup/info.ersn.net/server/internal/clients/caltrans"
	"github.com/dpup/info.ersn.net/server/internal/clients/google"
	"github.com/dpup/info.ersn.net/server/internal/clients/weather"
	"github.com/dpup/info.ersn.net/server/internal/config"
	"github.com/dpup/info.ersn.net/server/internal/services"
)

func main() {
	// Load configuration using Prefab's config system
	appConfig := loadConfig()

	// Initialize cache
	cache := cache.NewCache()

	// Initialize external API clients
	googleClient := google.NewClient(appConfig.Roads.GoogleRoutes.APIKey)
	caltransClient := caltrans.NewFeedParser()
	weatherClient := weather.NewClient(appConfig.Weather.OpenWeatherAPIKey)

	// Initialize gRPC services
	roadsService := services.NewRoadsService(googleClient, caltransClient, cache, &appConfig.Roads)
	weatherService := services.NewWeatherService(weatherClient, cache, &appConfig.Weather)

	log.Printf("Live Data API Server starting")
	log.Printf("Roads monitored: %d", len(appConfig.Roads.MonitoredRoads))
	log.Printf("Weather locations: %d", len(appConfig.Weather.Locations))

	// Create Prefab server with GRPC reflection enabled
	// Server configuration (port, etc.) will be loaded from prefab.yaml/env vars
	server := prefab.New(
		prefab.WithGRPCReflection(),
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