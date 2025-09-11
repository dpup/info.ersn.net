package main

import (
	"log"
	"os"
	"strconv"

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
	// Load configuration from environment variables
	googleAPIKey := os.Getenv("GOOGLE_API_KEY")
	if googleAPIKey == "" {
		googleAPIKey = os.Getenv("GOOGLE_ROUTES_API_KEY") // fallback
	}
	openweatherAPIKey := os.Getenv("OPENWEATHER_API_KEY")
	port := getEnvInt("PORT", 8080)

	// Create default configuration
	appConfig := createDefaultConfig(googleAPIKey, openweatherAPIKey)

	// Initialize cache
	cache := cache.NewCache()

	// Initialize external API clients
	googleClient := google.NewClient(googleAPIKey)
	caltransClient := caltrans.NewFeedParser()
	weatherClient := weather.NewClient(openweatherAPIKey)

	// Initialize gRPC services
	roadsService := services.NewRoadsService(googleClient, caltransClient, cache, &appConfig.Roads)
	weatherService := services.NewWeatherService(weatherClient, cache, &appConfig.Weather)

	log.Printf("Live Data API Server starting")
	log.Printf("Roads monitored: %d", len(appConfig.Roads.MonitoredRoads))
	log.Printf("Weather locations: %d", len(appConfig.Weather.Locations))
	log.Printf("Port: %d", port)

	// Create Prefab server with options
	server := prefab.New(
		prefab.WithPort(port),
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

// getEnvInt gets an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// createDefaultConfig creates a default configuration with API keys
func createDefaultConfig(googleAPIKey, openweatherAPIKey string) *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Port:        8080,
			CorsOrigins: []string{"*"},
		},
		Roads: config.RoadsConfig{
			GoogleRoutes: config.GoogleConfig{
				APIKey: googleAPIKey,
			},
			MonitoredRoads: []config.MonitoredRoad{
				{
					Name:    "Hwy 4",
					Section: "Angels Camp to Murphys",
					ID:      "hwy4-angels-murphys",
					Origin:      config.CoordinatesYAML{Latitude: 38.067400, Longitude: -120.540200},
					Destination: config.CoordinatesYAML{Latitude: 38.139117, Longitude: -120.456111},
				},
				{
					Name:    "Hwy 4",
					Section: "Murphys to Arnold",
					ID:      "hwy4-murphys-arnold",
					Origin:      config.CoordinatesYAML{Latitude: 38.139117, Longitude: -120.456111},
					Destination: config.CoordinatesYAML{Latitude: 38.265006, Longitude: -120.333654},
				},
				{
					Name:    "Hwy 4",
					Section: "Arnold to Bear Valley",
					ID:      "hwy4-arnold-bearvalley",
					Origin:      config.CoordinatesYAML{Latitude: 38.265006, Longitude: -120.333654},
					Destination: config.CoordinatesYAML{Latitude: 38.461045, Longitude: -120.042368},
				},
			},
		},
		Weather: config.WeatherConfig{
			OpenWeatherAPIKey: openweatherAPIKey,
			Locations: []config.WeatherLocation{
				{ID: "murphys", Name: "Murphys, CA", Lat: 38.139117, Lon: -120.456111},
				{ID: "arnold", Name: "Arnold, CA", Lat: 38.265006, Lon: -120.333654},
				{ID: "dorrington", Name: "Dorrington, CA", Lat: 38.301275, Lon: -120.276705},
				{ID: "bearvalley", Name: "Bear Valley, CA", Lat: 38.461045, Lon: -120.042368},
			},
		},
	}
}