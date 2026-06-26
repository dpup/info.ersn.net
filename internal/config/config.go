package config

import (
	"log"
	"time"

	"github.com/dpup/prefab"

	api "github.com/dpup/info.ersn.net/server/api/v1"
)

// Config represents the complete server configuration
type Config struct {
	GoogleRoutes GoogleRoutesClient `koanf:"googleRoutes"`
	OpenAI       OpenAIClient       `koanf:"openai"`
	OpenWeather  OpenWeatherClient  `koanf:"openweather"`
	Roads        RoadsConfig        `koanf:"roads"`
	Weather      WeatherConfig      `koanf:"weather"`
}

// RefreshConfig holds common refresh timing settings
type RefreshConfig struct {
	RefreshInterval time.Duration `koanf:"refreshInterval"`
	StaleThreshold  time.Duration `koanf:"staleThreshold"`
}

// Client configurations - moved to top level
type GoogleRoutesClient struct {
	APIKey string `koanf:"apiKey"`
}

type OpenAIClient struct {
	APIKey     string        `koanf:"apiKey"`
	Model      string        `koanf:"model"`
	Timeout    time.Duration `koanf:"timeout"`
	MaxRetries int           `koanf:"maxRetries"`
}

type OpenWeatherClient struct {
	APIKey string `koanf:"apiKey"`
}

// RoadsConfig holds road monitoring configuration
type RoadsConfig struct {
	CaltransFeeds   CaltransConfig  `koanf:"caltransFeeds"`
	MonitoredRoads  []MonitoredRoad `koanf:"monitoredRoads"`
	IncidentAreas   []IncidentArea  `koanf:"incidentAreas"`
	RefreshInterval time.Duration   `koanf:"refreshInterval"`
	StaleThreshold  time.Duration   `koanf:"staleThreshold"`
}

// IncidentArea defines a named geographic region for the region-wide incidents
// feed (GET /api/v1/incidents?area=<id>). Incidents whose coordinates fall
// inside Bounds are included.
type IncidentArea struct {
	ID     string    `koanf:"id"`
	Name   string    `koanf:"name"`
	Bounds GeoBounds `koanf:"bounds"`
}

// GeoBounds is an axis-aligned latitude/longitude bounding box.
type GeoBounds struct {
	MinLatitude  float64 `koanf:"minLatitude"`
	MaxLatitude  float64 `koanf:"maxLatitude"`
	MinLongitude float64 `koanf:"minLongitude"`
	MaxLongitude float64 `koanf:"maxLongitude"`
}

// Contains reports whether the given coordinate falls within the bounds.
func (b GeoBounds) Contains(lat, lon float64) bool {
	return lat >= b.MinLatitude && lat <= b.MaxLatitude &&
		lon >= b.MinLongitude && lon <= b.MaxLongitude
}

// CaltransConfig holds Caltrans KML feed settings
type CaltransConfig struct {
	LaneClosures CaltransFeedConfig `koanf:"laneClosures"`
	CHPIncidents CaltransFeedConfig `koanf:"chpIncidents"`
}

// CaltransFeedConfig holds individual feed configuration
type CaltransFeedConfig struct {
	RefreshInterval time.Duration `koanf:"refreshInterval"`
	URL             string        `koanf:"url"`
}

// MonitoredRoad represents a road to monitor
type MonitoredRoad struct {
	Name        string      `koanf:"name"`
	Section     string      `koanf:"section"`
	ID          string      `koanf:"id"`
	Origin      Coordinates `koanf:"origin"`
	Destination Coordinates `koanf:"destination"`
}

// WeatherConfig holds weather monitoring configuration
type WeatherConfig struct {
	Locations       []WeatherLocation `koanf:"locations"`
	NWS             NWSConfig         `koanf:"nws"`
	RefreshInterval time.Duration     `koanf:"refreshInterval"`
	StaleThreshold  time.Duration     `koanf:"staleThreshold"`
}

// NWSConfig holds National Weather Service (api.weather.gov) settings used for
// authoritative zone alerts (issue #4) and fire-weather classification (issue #5).
type NWSConfig struct {
	// UserAgent identifies the app to api.weather.gov (required by NWS).
	UserAgent string `koanf:"userAgent"`
	// Zones is the set of NWS forecast zones covering the service area
	// (e.g. CAZ064, CAZ065, CAZ258, CAZ259).
	Zones []string `koanf:"zones"`
}

// WeatherLocation represents a location to monitor for weather
type WeatherLocation struct {
	ID          string      `koanf:"id"`
	Name        string      `koanf:"name"`
	Coordinates Coordinates `koanf:"coordinates"`
}

// Coordinates represents lat/lon coordinates - unified structure
type Coordinates struct {
	Latitude  float64 `koanf:"latitude"`
	Longitude float64 `koanf:"longitude"`
}

// ToProto converts Coordinates to protobuf Coordinates
func (c Coordinates) ToProto() *api.Coordinates {
	return &api.Coordinates{
		Latitude:  c.Latitude,
		Longitude: c.Longitude,
	}
}

// ToProto converts WeatherLocation to protobuf Coordinates
func (w WeatherLocation) ToProto() *api.Coordinates {
	return w.Coordinates.ToProto()
}

// LoadConfig loads configuration using Prefab's config system
// Configuration is loaded from prefab.yaml and environment variables with PF__ prefix
func LoadConfig() *Config {
	appConfig := &Config{}
	// Unmarshal client configurations
	if err := prefab.Config.Unmarshal("googleRoutes", &appConfig.GoogleRoutes); err != nil {
		log.Fatalf("Failed to unmarshal googleRoutes section: %v", err)
	}
	if err := prefab.Config.Unmarshal("openai", &appConfig.OpenAI); err != nil {
		log.Fatalf("Failed to unmarshal openai section: %v", err)
	}
	if err := prefab.Config.Unmarshal("openweather", &appConfig.OpenWeather); err != nil {
		log.Fatalf("Failed to unmarshal openweather section: %v", err)
	}
	// Unmarshal service configurations
	if err := prefab.Config.Unmarshal("roads", &appConfig.Roads); err != nil {
		log.Fatalf("Failed to unmarshal roads section: %v", err)
	}
	if err := prefab.Config.Unmarshal("weather", &appConfig.Weather); err != nil {
		log.Fatalf("Failed to unmarshal weather section: %v", err)
	}
	return appConfig
}
