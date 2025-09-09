package config

import (
	"time"

	api "github.com/dpup/info.ersn.net/server"
)

// Config represents the complete server configuration
// Structure matches research.md YAML configuration (lines 112-146)
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Routes  RoutesConfig  `yaml:"routes"`
	Weather WeatherConfig `yaml:"weather"`
}

// ServerConfig holds server-specific settings
type ServerConfig struct {
	Port        int      `yaml:"port"`
	CorsOrigins []string `yaml:"cors_origins"`
}

// RoutesConfig holds route monitoring configuration
type RoutesConfig struct {
	GoogleRoutes    GoogleConfig       `yaml:"google_routes"`
	CaltransFeeds   CaltransConfig     `yaml:"caltrans_feeds"`
	MonitoredRoutes []MonitoredRoute   `yaml:"monitored_routes"`
}

// GoogleConfig holds Google Routes API settings
type GoogleConfig struct {
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	StaleThreshold  time.Duration `yaml:"stale_threshold"`
	APIKey          string        `yaml:"api_key"`
}

// CaltransConfig holds Caltrans KML feed settings
type CaltransConfig struct {
	ChainControls CaltransFeedConfig `yaml:"chain_controls"`
	LaneClosures  CaltransFeedConfig `yaml:"lane_closures"`
	CHPIncidents  CaltransFeedConfig `yaml:"chp_incidents"`
}

// CaltransFeedConfig holds individual feed configuration
type CaltransFeedConfig struct {
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	URL             string        `yaml:"url"`
}

// MonitoredRoute represents a route to monitor
type MonitoredRoute struct {
	Name        string           `yaml:"name"`
	ID          string           `yaml:"id"`
	Origin      CoordinatesYAML  `yaml:"origin"`
	Destination CoordinatesYAML  `yaml:"destination"`
}

// WeatherConfig holds weather monitoring configuration  
type WeatherConfig struct {
	RefreshInterval    time.Duration     `yaml:"refresh_interval"`
	StaleThreshold     time.Duration     `yaml:"stale_threshold"`
	OpenWeatherAPIKey  string            `yaml:"openweather_api_key"`
	Locations          []WeatherLocation `yaml:"locations"`
}

// WeatherLocation represents a location to monitor for weather
type WeatherLocation struct {
	ID   string  `yaml:"id"`
	Name string  `yaml:"name"`
	Lat  float64 `yaml:"lat"`
	Lon  float64 `yaml:"lon"`
}

// CoordinatesYAML represents lat/lon coordinates in YAML config
type CoordinatesYAML struct {
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
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

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:        8080,
			CorsOrigins: []string{"*"},
		},
		Routes: RoutesConfig{
			GoogleRoutes: GoogleConfig{
				RefreshInterval: 5 * time.Minute,
				StaleThreshold:  10 * time.Minute,
			},
			CaltransFeeds: CaltransConfig{
				ChainControls: CaltransFeedConfig{
					RefreshInterval: 15 * time.Minute, // Less frequent, changes slowly
					URL:             "https://quickmap.dot.ca.gov/data/cc.kml",
				},
				LaneClosures: CaltransFeedConfig{
					RefreshInterval: 10 * time.Minute,
					URL:             "https://quickmap.dot.ca.gov/data/lcs2way.kml",
				},
				CHPIncidents: CaltransFeedConfig{
					RefreshInterval: 5 * time.Minute, // More frequent, incidents change quickly
					URL:             "https://quickmap.dot.ca.gov/data/chp-only.kml",
				},
			},
			MonitoredRoutes: []MonitoredRoute{
				{
					Name: "I-5 Seattle to Portland",
					ID:   "i5-sea-pdx",
					Origin: CoordinatesYAML{
						Latitude:  47.6062,
						Longitude: -122.3321,
					},
					Destination: CoordinatesYAML{
						Latitude:  45.5152,
						Longitude: -122.6784,
					},
				},
			},
		},
		Weather: WeatherConfig{
			RefreshInterval: 5 * time.Minute,
			StaleThreshold:  10 * time.Minute,
			Locations: []WeatherLocation{
				{
					ID:   "seattle",
					Name: "Seattle, WA",
					Lat:  47.6062,
					Lon:  -122.3321,
				},
				{
					ID:   "portland",
					Name: "Portland, OR",
					Lat:  45.5152,
					Lon:  -122.6784,
				},
			},
		},
	}
}