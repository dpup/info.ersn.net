package config

import (
	"time"

	api "github.com/dpup/info.ersn.net/server/api/v1"
)

// Config represents the complete server configuration
// Structure matches research.md YAML configuration (lines 112-146)
type Config struct {
	Server  ServerConfig  `yaml:"server" koanf:"server"`
	Roads   RoadsConfig   `yaml:"roads" koanf:"roads"`
	Weather WeatherConfig `yaml:"weather" koanf:"weather"`
}

// ServerConfig holds server-specific settings
type ServerConfig struct {
	Port        int      `yaml:"port" koanf:"port"`
	CorsOrigins []string `yaml:"cors_origins" koanf:"cors_origins"`
}

// RoadsConfig holds road monitoring configuration
type RoadsConfig struct {
	GoogleRoutes   GoogleConfig      `yaml:"google_routes" koanf:"google_routes"`
	CaltransFeeds  CaltransConfig    `yaml:"caltrans_feeds" koanf:"caltrans_feeds"`
	MonitoredRoads []MonitoredRoad   `yaml:"monitored_roads" koanf:"monitored_roads"`
}

// GoogleConfig holds Google Routes API settings
type GoogleConfig struct {
	RefreshInterval time.Duration `yaml:"refresh_interval" koanf:"refresh_interval"`
	StaleThreshold  time.Duration `yaml:"stale_threshold" koanf:"stale_threshold"`
	APIKey          string        `yaml:"api_key" koanf:"api_key"`
}

// CaltransConfig holds Caltrans KML feed settings
type CaltransConfig struct {
	ChainControls CaltransFeedConfig `yaml:"chain_controls" koanf:"chain_controls"`
	LaneClosures  CaltransFeedConfig `yaml:"lane_closures" koanf:"lane_closures"`
	CHPIncidents  CaltransFeedConfig `yaml:"chp_incidents" koanf:"chp_incidents"`
}

// CaltransFeedConfig holds individual feed configuration
type CaltransFeedConfig struct {
	RefreshInterval time.Duration `yaml:"refresh_interval" koanf:"refresh_interval"`
	URL             string        `yaml:"url" koanf:"url"`
}

// MonitoredRoad represents a road to monitor
type MonitoredRoad struct {
	Name        string           `yaml:"name" koanf:"name"`
	Section     string           `yaml:"section" koanf:"section"`
	ID          string           `yaml:"id" koanf:"id"`
	Origin      CoordinatesYAML  `yaml:"origin" koanf:"origin"`
	Destination CoordinatesYAML  `yaml:"destination" koanf:"destination"`
}

// WeatherConfig holds weather monitoring configuration  
type WeatherConfig struct {
	RefreshInterval    time.Duration     `yaml:"refresh_interval" koanf:"refresh_interval"`
	StaleThreshold     time.Duration     `yaml:"stale_threshold" koanf:"stale_threshold"`
	OpenWeatherAPIKey  string            `yaml:"openweather_api_key" koanf:"openweather_api_key"`
	Locations          []WeatherLocation `yaml:"locations" koanf:"locations"`
}

// WeatherLocation represents a location to monitor for weather
type WeatherLocation struct {
	ID   string  `yaml:"id" koanf:"id"`
	Name string  `yaml:"name" koanf:"name"`
	Lat  float64 `yaml:"lat" koanf:"lat"`
	Lon  float64 `yaml:"lon" koanf:"lon"`
}

// CoordinatesYAML represents lat/lon coordinates in YAML config
type CoordinatesYAML struct {
	Latitude  float64 `yaml:"latitude" koanf:"latitude"`
	Longitude float64 `yaml:"longitude" koanf:"longitude"`
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
		Roads: RoadsConfig{
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
			MonitoredRoads: []MonitoredRoad{
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