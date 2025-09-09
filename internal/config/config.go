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
					Name: "Hwy 4 - Angels Camp to Murphys",
					ID:   "hwy4-angels-murphys",
					Origin: CoordinatesYAML{
						Latitude:  38.0674,
						Longitude: -120.5402,
					},
					Destination: CoordinatesYAML{
						Latitude:  38.1327,
						Longitude: -120.4606,
					},
				},
				{
					Name: "Hwy 4 - Murphys to Arnold",
					ID:   "hwy4-murphys-arnold",
					Origin: CoordinatesYAML{
						Latitude:  38.1327,
						Longitude: -120.4606,
					},
					Destination: CoordinatesYAML{
						Latitude:  38.2458,
						Longitude: -120.3486,
					},
				},
				{
					Name: "Hwy 4 - Arnold to Ebbetts Pass",
					ID:   "hwy4-arnold-ebbetts",
					Origin: CoordinatesYAML{
						Latitude:  38.2458,
						Longitude: -120.3486,
					},
					Destination: CoordinatesYAML{
						Latitude:  38.5347,
						Longitude: -119.8075,
					},
				},
			},
		},
		Weather: WeatherConfig{
			RefreshInterval: 5 * time.Minute,
			StaleThreshold:  10 * time.Minute,
			Locations: []WeatherLocation{
				{
					ID:   "murphys",
					Name: "Murphys, CA",
					Lat:  38.1327,
					Lon:  -120.4606,
				},
				{
					ID:   "arnold",
					Name: "Arnold, CA",
					Lat:  38.2458,
					Lon:  -120.3486,
				},
				{
					ID:   "dorrington",
					Name: "Dorrington, CA",
					Lat:  38.3461,
					Lon:  -120.2036,
				},
			},
		},
	}
}