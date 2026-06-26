package hazards

import "strings"

// Layer identifiers (docs/hazard-aggregation-design.md §4.4).
const (
	LayerRoadIncident = "road_incident"
	LayerRoadSegment  = "road_segment"
	LayerChainControl = "chain_control"
	LayerWeatherAlert = "weather_alert"
	LayerFireWeather  = "fire_weather"
	// Layers added in later milestones: wildfire, evacuation, earthquake.
)

// Properties is the common envelope shared by every hazard feature, plus a
// namespaced per-kind block (only the relevant one is set).
type Properties struct {
	ID           string `json:"id"`
	Layer        string `json:"layer"`
	Kind         string `json:"kind"`
	Category     string `json:"category,omitempty"`
	Severity     string `json:"severity"`
	SeverityRank int    `json:"severity_rank"`
	Headline     string `json:"headline"`
	Description  string `json:"description,omitempty"`
	Status       string `json:"status,omitempty"`
	Effective    string `json:"effective,omitempty"`
	Expires      string `json:"expires,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
	AreaLabel    string `json:"area_label,omitempty"`
	Source       Source `json:"source"`

	// Per-kind typed blocks (exactly one populated).
	Incident     *IncidentProps     `json:"incident,omitempty"`
	Road         *RoadProps         `json:"road,omitempty"`
	ChainControl *ChainControlProps `json:"chain_control,omitempty"`
	Weather      *WeatherProps      `json:"weather,omitempty"`
	FireWeather  *FireWeatherProps  `json:"fire_weather,omitempty"`
}

// Source identifies the upstream feed a feature came from.
type Source struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url,omitempty"`
	Attribution string `json:"attribution,omitempty"`
	FetchedAt   string `json:"fetched_at,omitempty"`
}

// IncidentProps is the road_incident kind block.
type IncidentProps struct {
	LogNumber string `json:"log_number,omitempty"`
}

// RoadProps is the road_segment kind block.
type RoadProps struct {
	RoadID          string `json:"road_id"`
	Congestion      string `json:"congestion,omitempty"`
	DelayMinutes    int32  `json:"delay_minutes"`
	DurationMinutes int32  `json:"duration_minutes,omitempty"`
	DistanceKm      int32  `json:"distance_km,omitempty"`
}

// ChainControlProps is the chain_control kind block.
type ChainControlProps struct {
	Level     string `json:"level,omitempty"` // R1 | R2 | R3
	Highway   string `json:"highway,omitempty"`
	Direction string `json:"direction,omitempty"`
}

// WeatherProps is the weather_alert kind block.
type WeatherProps struct {
	Event  string   `json:"event,omitempty"`
	Source string   `json:"source,omitempty"` // NWS | OPENWEATHERMAP
	Zones  []string `json:"zones,omitempty"`
}

// FireWeatherProps is the fire_weather kind block.
type FireWeatherProps struct {
	State       string   `json:"state"` // normal | elevated | red-flag
	SourceEvent string   `json:"source_event,omitempty"`
	Zones       []string `json:"zones,omitempty"`
}

// setSeverity sets both Severity and the derived SeverityRank.
func (p *Properties) setSeverity(s string) {
	p.Severity = s
	p.SeverityRank = severityRank(s)
}

// safeURL returns u only if it is an http(s) URL; upstream data is untrusted and
// a javascript:/data: URL rendered in a popup is an XSS/open-redirect vector
// (design §4.1).
func safeURL(u string) string {
	u = strings.TrimSpace(u)
	if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
		return u
	}
	return ""
}
