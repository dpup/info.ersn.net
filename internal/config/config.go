package config

import (
	"time"

	api "github.com/dpup/info.ersn.net/server/api/v1"
)

// Config represents the complete server configuration
type Config struct {
	Server  ServerConfig  `koanf:"server"`
	Roads   RoadsConfig   `koanf:"roads"`
	Weather WeatherConfig `koanf:"weather"`
}

// ServerConfig holds server-specific settings
type ServerConfig struct {
	Port        int      `koanf:"port"`
	CorsOrigins []string `koanf:"cors_origins"`
}

// RoadsConfig holds road monitoring configuration
type RoadsConfig struct {
	GoogleRoutes   GoogleConfig      `koanf:"google_routes"`
	CaltransFeeds  CaltransConfig    `koanf:"caltrans_feeds"`
	MonitoredRoads []MonitoredRoad   `koanf:"monitored_roads"`
}

// GoogleConfig holds Google Routes API settings
type GoogleConfig struct {
	RefreshInterval time.Duration `koanf:"refresh_interval"`
	StaleThreshold  time.Duration `koanf:"stale_threshold"`
	APIKey          string        `koanf:"api_key"`
}

// CaltransConfig holds Caltrans KML feed settings
type CaltransConfig struct {
	ChainControls CaltransFeedConfig `koanf:"chain_controls"`
	LaneClosures  CaltransFeedConfig `koanf:"lane_closures"`
	CHPIncidents  CaltransFeedConfig `koanf:"chp_incidents"`
}

// CaltransFeedConfig holds individual feed configuration
type CaltransFeedConfig struct {
	RefreshInterval time.Duration `koanf:"refresh_interval"`
	URL             string        `koanf:"url"`
}

// MonitoredRoad represents a road to monitor
type MonitoredRoad struct {
	Name        string           `koanf:"name"`
	Section     string           `koanf:"section"`
	ID          string           `koanf:"id"`
	Origin      CoordinatesYAML  `koanf:"origin"`
	Destination CoordinatesYAML  `koanf:"destination"`
}

// WeatherConfig holds weather monitoring configuration  
type WeatherConfig struct {
	RefreshInterval    time.Duration     `koanf:"refresh_interval"`
	StaleThreshold     time.Duration     `koanf:"stale_threshold"`
	OpenWeatherAPIKey  string            `koanf:"openweather_api_key"`
	Locations          []WeatherLocation `koanf:"locations"`
}

// WeatherLocation represents a location to monitor for weather
type WeatherLocation struct {
	ID   string  `koanf:"id"`
	Name string  `koanf:"name"`
	Lat  float64 `koanf:"lat"`
	Lon  float64 `koanf:"lon"`
}

// CoordinatesYAML represents lat/lon coordinates in config
type CoordinatesYAML struct {
	Latitude  float64 `koanf:"latitude"`
	Longitude float64 `koanf:"longitude"`
}

// ToProto converts CoordinatesYAML to protobuf Coordinates
func (c CoordinatesYAML) ToProto() *api.Coordinates {
	return &api.Coordinates{
		Latitude:  c.Latitude,
		Longitude: c.Longitude,
	}
}

// ToProto converts WeatherLocation to protobuf Coordinates
func (w WeatherLocation) ToProto() *api.Coordinates {
	return &api.Coordinates{
		Latitude:  w.Lat,
		Longitude: w.Lon,
	}
}

