package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/dpup/prefab/logging"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/cache"
	"github.com/dpup/info.ersn.net/server/internal/clients/weather"
	"github.com/dpup/info.ersn.net/server/internal/config"
	"github.com/dpup/info.ersn.net/server/internal/lib/alerts"
)

// WeatherService implements the gRPC WeatherService
// Implementation per tasks.md T017 and data-model.md WeatherData entity
type WeatherService struct {
	api.UnimplementedWeatherServiceServer
	weatherClient  *weather.Client
	cache          *cache.Cache
	config         *config.Config
	alertEnhancer  alerts.WeatherAlertEnhancer
}

// NewWeatherService creates a new WeatherService
func NewWeatherService(weatherClient *weather.Client, cache *cache.Cache, config *config.Config, alertEnhancer alerts.WeatherAlertEnhancer) *WeatherService {
	return &WeatherService{
		weatherClient:  weatherClient,
		cache:          cache,
		config:         config,
		alertEnhancer:  alertEnhancer,
	}
}

// ListWeather implements the gRPC method defined in contracts/weather.proto lines 12-17
func (s *WeatherService) ListWeather(ctx context.Context, req *api.ListWeatherRequest) (*api.ListWeatherResponse, error) {
	logging.Info(ctx, "ListWeather called")

	// Try to get cached weather data first
	var cachedWeatherData []*api.WeatherData
	cacheKey := "weather:all"
	
	found, err := s.cache.Get(cacheKey, &cachedWeatherData)
	if err != nil {
		logging.Errorw(ctx, "Cache error", "error", err, "cache_key", cacheKey)
	}

	if found && !s.cache.IsStale(cacheKey) {
		logging.Infow(ctx, "Returning cached weather data", "location_count", len(cachedWeatherData))
		
		// Get cache metadata for last_updated timestamp
		entry, _, _ := s.cache.GetWithMetadata(cacheKey, nil)
		var lastUpdated *timestamppb.Timestamp
		if entry != nil {
			lastUpdated = timestamppb.New(entry.CreatedAt)
		}

		return &api.ListWeatherResponse{
			WeatherData: cachedWeatherData,
			LastUpdated: lastUpdated,
		}, nil
	}

	// Cache miss or stale - refresh from external API
	logging.Info(ctx, "Refreshing weather data from OpenWeatherMap API")
	weatherData, err := s.refreshWeatherData(ctx)
	if err != nil {
		// If refresh fails but we have stale cached data, return it
		if found && !s.cache.IsVeryStale(cacheKey) {
			logging.Errorw(ctx, "Refresh failed, returning stale cached weather data", "error", err)
			entry, _, _ := s.cache.GetWithMetadata(cacheKey, nil)
			var lastUpdated *timestamppb.Timestamp
			if entry != nil {
				lastUpdated = timestamppb.New(entry.CreatedAt)
			}

			return &api.ListWeatherResponse{
				WeatherData: cachedWeatherData,
				LastUpdated: lastUpdated,
			}, nil
		}
		return nil, fmt.Errorf("failed to refresh weather data: %w", err)
	}

	// Cache the refreshed data
	if err := s.cache.Set(cacheKey, weatherData, s.config.Weather.RefreshInterval, "weather"); err != nil {
		logging.Errorw(ctx, "Failed to cache weather data", "error", err)
	}

	return &api.ListWeatherResponse{
		WeatherData: weatherData,
		LastUpdated: timestamppb.Now(),
	}, nil
}

// GetLocationWeather implements the gRPC method for retrieving weather for a specific location
func (s *WeatherService) GetLocationWeather(ctx context.Context, req *api.GetLocationWeatherRequest) (*api.GetLocationWeatherResponse, error) {
	logging.Infow(ctx, "GetLocationWeather called", "location_id", req.LocationId)

	// Get all weather data (will use cache if available)
	listResp, err := s.ListWeather(ctx, &api.ListWeatherRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get weather data: %w", err)
	}

	// Find the requested location
	for _, weatherData := range listResp.WeatherData {
		if weatherData.LocationId == req.LocationId {
			return &api.GetLocationWeatherResponse{
				WeatherData: weatherData,
				LastUpdated: listResp.LastUpdated,
			}, nil
		}
	}

	return nil, fmt.Errorf("location not found: %s", req.LocationId)
}

// ListWeatherAlerts implements the gRPC method for retrieving weather alerts
func (s *WeatherService) ListWeatherAlerts(ctx context.Context, req *api.ListWeatherAlertsRequest) (*api.ListWeatherAlertsResponse, error) {
	logging.Info(ctx, "ListWeatherAlerts called")

	// Try to get cached alerts first
	var cachedAlerts []*api.WeatherAlert
	cacheKey := "weather:alerts"
	
	found, err := s.cache.Get(cacheKey, &cachedAlerts)
	if err != nil {
		logging.Errorw(ctx, "Cache error", "error", err, "cache_key", cacheKey)
	}

	if found && !s.cache.IsStale(cacheKey) {
		logging.Infow(ctx, "Returning cached weather alerts", "alert_count", len(cachedAlerts))
		
		entry, _, _ := s.cache.GetWithMetadata(cacheKey, nil)
		var lastUpdated *timestamppb.Timestamp
		if entry != nil {
			lastUpdated = timestamppb.New(entry.CreatedAt)
		}

		return &api.ListWeatherAlertsResponse{
			Alerts:      cachedAlerts,
			LastUpdated: lastUpdated,
		}, nil
	}

	// Cache miss or stale - refresh alerts from external API
	logging.Info(ctx, "Refreshing weather alerts from OpenWeatherMap API")
	alerts, err := s.refreshWeatherAlerts(ctx)
	if err != nil {
		// If refresh fails but we have stale cached data, return it
		if found && !s.cache.IsVeryStale(cacheKey) {
			logging.Errorw(ctx, "Refresh failed, returning stale cached alerts", "error", err)
			entry, _, _ := s.cache.GetWithMetadata(cacheKey, nil)
			var lastUpdated *timestamppb.Timestamp
			if entry != nil {
				lastUpdated = timestamppb.New(entry.CreatedAt)
			}

			return &api.ListWeatherAlertsResponse{
				Alerts:      cachedAlerts,
				LastUpdated: lastUpdated,
			}, nil
		}
		return nil, fmt.Errorf("failed to refresh weather alerts: %w", err)
	}

	// Cache the refreshed alerts
	if err := s.cache.Set(cacheKey, alerts, s.config.Weather.RefreshInterval, "weather_alerts"); err != nil {
		logging.Errorw(ctx, "Failed to cache weather alerts", "error", err)
	}

	return &api.ListWeatherAlertsResponse{
		Alerts:      alerts,
		LastUpdated: timestamppb.Now(),
	}, nil
}

// refreshWeatherData fetches fresh weather data from OpenWeatherMap for all configured locations
func (s *WeatherService) refreshWeatherData(ctx context.Context) ([]*api.WeatherData, error) {
	var weatherDataList []*api.WeatherData

	// Get existing cached data to preserve on per-location failures
	var existingData []*api.WeatherData
	existingDataMap := make(map[string]*api.WeatherData)
	cacheKey := "weather:all"
	if found, _ := s.cache.Get(cacheKey, &existingData); found {
		for _, wd := range existingData {
			existingDataMap[wd.LocationId] = wd
		}
	}

	logging.Infow(ctx, "Starting weather refresh", "location_count", len(s.config.Weather.Locations))

	// Process each configured location
	for i, location := range s.config.Weather.Locations {
		logging.Infow(ctx, "Processing weather location", "index", i, "location_id", location.ID, "location_name", location.Name)

		weatherData, err := s.processWeatherLocation(ctx, location)
		if err != nil {
			logging.Errorw(ctx, "Failed to process weather for location",
				"location_id", location.ID,
				"location_name", location.Name,
				"error", err)
			// Try to preserve existing cached data for this location
			if existing, ok := existingDataMap[location.ID]; ok {
				logging.Infow(ctx, "Preserving stale weather data for location", "location_id", location.ID)
				weatherDataList = append(weatherDataList, existing)
			}
			continue
		}
		weatherDataList = append(weatherDataList, weatherData)
		logging.Infow(ctx, "Successfully processed weather location", "location_id", location.ID)
	}

	logging.Infow(ctx, "Weather refresh complete",
		"total_locations", len(s.config.Weather.Locations),
		"successful_locations", len(weatherDataList))

	if len(weatherDataList) == 0 {
		return nil, fmt.Errorf("no weather data could be processed")
	}

	return weatherDataList, nil
}

// processWeatherLocation fetches weather data for a single location
func (s *WeatherService) processWeatherLocation(ctx context.Context, location config.WeatherLocation) (*api.WeatherData, error) {
	logging.Infow(ctx, "Processing weather for location", "location_id", location.ID)

	if s.config.OpenWeather.APIKey == "" {
		return nil, fmt.Errorf("OpenWeatherMap API key not configured")
	}

	// Get current weather data
	weatherData, err := s.weatherClient.GetCurrentWeather(ctx, location.ToProto())
	if err != nil {
		return nil, fmt.Errorf("failed to get current weather: %w", err)
	}

	// Set location ID and name from config
	weatherData.LocationId = location.ID
	weatherData.LocationName = location.Name

	// Get weather alerts for this location
	locationAlerts, err := s.weatherClient.GetWeatherAlerts(ctx, location.ToProto())
	if err != nil {
		logging.Errorw(ctx, "Failed to get weather alerts", "location_id", location.ID, "error", err)
		// Continue without alerts rather than failing
		locationAlerts = nil
	}

	// Enhance alerts with AI if enhancer is available
	for _, alert := range locationAlerts {
		alert.Id = fmt.Sprintf("%s_%s", location.ID, alert.Id)
		if s.alertEnhancer != nil {
			s.enhanceWeatherAlert(ctx, alert)
		}
	}

	weatherData.Alerts = locationAlerts

	return weatherData, nil
}

// refreshWeatherAlerts fetches fresh weather alerts from OpenWeatherMap for all configured locations
func (s *WeatherService) refreshWeatherAlerts(ctx context.Context) ([]*api.WeatherAlert, error) {
	var allAlerts []*api.WeatherAlert

	// Process each configured location
	for _, location := range s.config.Weather.Locations {
		locationAlerts, err := s.weatherClient.GetWeatherAlerts(ctx, location.ToProto())
		if err != nil {
			logging.Errorw(ctx, "Failed to get weather alerts for location", "location_id", location.ID, "error", err)
			// Continue processing other locations even if one fails
			continue
		}

		// Add location context to alert IDs and enhance each alert
		for _, alert := range locationAlerts {
			alert.Id = fmt.Sprintf("%s_%s", location.ID, alert.Id)

			// Enhance the alert with AI if enhancer is available
			if s.alertEnhancer != nil {
				s.enhanceWeatherAlert(ctx, alert)
			}
		}

		allAlerts = append(allAlerts, locationAlerts...)
	}

	return allAlerts, nil
}

// enhanceWeatherAlert enhances a single weather alert with AI-generated content
// Uses content-based caching to avoid duplicate OpenAI calls
func (s *WeatherService) enhanceWeatherAlert(ctx context.Context, alert *api.WeatherAlert) {
	// Generate content hash for cache key
	contentHash := s.hashWeatherAlertContent(alert)
	cacheKey := fmt.Sprintf("weather_alert_enhanced:%s", contentHash)

	// Check cache first
	var cachedEnhancement alerts.EnhancedWeatherAlert
	if found, err := s.cache.Get(cacheKey, &cachedEnhancement); err == nil && found {
		logging.Infow(ctx, "Using cached weather alert enhancement", "hash", contentHash[:8])
		alert.Headline = cachedEnhancement.Headline
		alert.Summary = cachedEnhancement.Summary
		alert.Details = cachedEnhancement.Details
		return
	}

	// Cache miss - call OpenAI enhancement
	logging.Infow(ctx, "Enhancing weather alert with AI", "event", alert.Event, "hash", contentHash[:8])

	rawAlert := alerts.RawWeatherAlert{
		ID:          alert.Id,
		Event:       alert.Event,
		SenderName:  alert.SenderName,
		Description: alert.Description,
		Tags:        alert.Tags,
		Start:       alert.StartTimestamp,
		End:         alert.EndTimestamp,
	}

	enhanced, err := s.alertEnhancer.EnhanceWeatherAlert(ctx, rawAlert)
	if err != nil {
		logging.Errorw(ctx, "Weather alert enhancement failed, using original", "error", err)
		// Fall back to using original description for all fields
		alert.Headline = alert.Event
		alert.Summary = s.truncateText(alert.Description, 200)
		alert.Details = alert.Description
		return
	}

	// Apply enhancement to alert
	alert.Headline = enhanced.Headline
	alert.Summary = enhanced.Summary
	alert.Details = enhanced.Details

	// Cache the enhancement with 24-hour TTL
	if err := s.cache.Set(cacheKey, enhanced, 24*time.Hour, "weather_alert_enhanced"); err != nil {
		logging.Errorw(ctx, "Failed to cache weather alert enhancement", "error", err)
	}
}

// hashWeatherAlertContent creates a content hash for weather alert deduplication
func (s *WeatherService) hashWeatherAlertContent(alert *api.WeatherAlert) string {
	// Weather alerts are identified by event type + description content
	// Timestamps change but content stays the same for the same alert
	contentSignature := fmt.Sprintf("%s|%s|%s",
		alert.Event,
		alert.SenderName,
		alert.Description,
	)
	hash := sha256.Sum256([]byte(contentSignature))
	return fmt.Sprintf("%x", hash)
}

// truncateText truncates text to a maximum length with ellipsis
func (s *WeatherService) truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}