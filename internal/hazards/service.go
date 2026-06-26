package hazards

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dpup/prefab/logging"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/clients/calfire"
	"github.com/dpup/info.ersn.net/server/internal/clients/caloes"
	"github.com/dpup/info.ersn.net/server/internal/clients/caltrans"
	"github.com/dpup/info.ersn.net/server/internal/clients/usgs"
	"github.com/dpup/info.ersn.net/server/internal/clients/wfigs"
	"github.com/dpup/info.ersn.net/server/internal/config"
	"github.com/dpup/info.ersn.net/server/internal/services"
)

// roadsAPI / weatherAPI are the slices of the existing services the hazard layer
// re-projects. Interfaces keep the package testable.
type roadsAPI interface {
	ListRoads(context.Context, *api.ListRoadsRequest) (*api.ListRoadsResponse, error)
	ListIncidents(context.Context, *api.ListIncidentsRequest) (*api.ListIncidentsResponse, error)
}
type weatherAPI interface {
	ListWeather(context.Context, *api.ListWeatherRequest) (*api.ListWeatherResponse, error)
	ListWeatherAlerts(context.Context, *api.ListWeatherAlertsRequest) (*api.ListWeatherAlertsResponse, error)
}

// Service re-projects the service's existing feeds into the unified GeoJSON
// hazard model and serves them at /api/v1/hazards/{area}/{layer}.geojson.
type Service struct {
	cfg      *config.Config
	roads    roadsAPI
	weather  weatherAPI
	caltrans *caltrans.FeedParser
	usgs     *usgs.Client
	calfire  *calfire.Client
	wfigs    *wfigs.Client
	caloes   *caloes.Client
}

// NewService wires the hazard service to the existing services + clients. The
// new-upstream clients (USGS, CAL FIRE, WFIGS, ...) are keyless and constructed
// here.
func NewService(cfg *config.Config, roads *services.RoadsService, weather *services.WeatherService, ct *caltrans.FeedParser) *Service {
	return &Service{
		cfg:      cfg,
		roads:    roads,
		weather:  weather,
		caltrans: ct,
		usgs:     usgs.NewClient(),
		calfire:  calfire.NewClient(),
		wfigs:    wfigs.NewClient(),
		caloes:   caloes.NewClient(),
	}
}

// HandlerPrefix is where the layer endpoints mount.
const HandlerPrefix = "/api/v1/hazards/"

// builder produces a layer's features for an area. Returning an error makes the
// layer fail-loud (UNAVAILABLE metadata, empty features) rather than fabricating
// a clear state.
type builder func(ctx context.Context, area config.HazardArea) ([]Feature, error)

func (s *Service) builders() map[string]builder {
	return map[string]builder{
		LayerRoadIncident: s.roadIncidents,
		LayerChainControl: s.chainControls,
		LayerRoadSegment:  s.roadSegments,
		LayerWeatherAlert: s.weatherAlerts,
		LayerFireWeather:  s.fireWeather,
		LayerEarthquake:   s.earthquakes,
		LayerWildfire:     s.wildfires,
		LayerEvacuation:   s.evacuations,
	}
}

// ServeHTTP handles GET /api/v1/hazards/{area}/{layer}.geojson.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, HandlerPrefix)
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || !strings.HasSuffix(parts[1], ".geojson") {
		http.Error(w, "not found: expected /api/v1/hazards/{area}/{layer}.geojson", http.StatusNotFound)
		return
	}
	areaID := parts[0]
	layer := strings.TrimSuffix(parts[1], ".geojson")

	area, ok := s.resolveArea(areaID)
	if !ok {
		http.Error(w, fmt.Sprintf("unknown hazard area: %q", areaID), http.StatusNotFound)
		return
	}
	build, ok := s.builders()[layer]
	if !ok {
		http.Error(w, fmt.Sprintf("unknown hazard layer: %q", layer), http.StatusNotFound)
		return
	}

	ctx := r.Context()
	res := s.buildLayer(ctx, area, layer, build)

	fc := newCollection(res.features, &Metadata{
		Layer:         layer,
		Area:          areaID,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		SourceStatus:  res.status,
		Attribution:   res.meta.attribution,
		SourceURL:     res.meta.sourceURL,
		SchemaVersion: schemaVersion,
	})

	w.Header().Set("Content-Type", "application/geo+json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	if err := json.NewEncoder(w).Encode(fc); err != nil {
		logging.Errorw(ctx, "Failed to encode hazard GeoJSON", "error", err)
	}
}

// ScannersPrefix is where the scanner-config endpoint mounts.
const ScannersPrefix = "/api/v1/scanners/"

// scannerOut is the response shape for GET /api/v1/scanners/{area}. Note: no
// raw HTML `embed` field (the client builds the official Broadcastify iframe
// from feed_id) — see the security review.
type scannerOut struct {
	FeedID          string `json:"feed_id"`
	ChannelLabel    string `json:"channel_label"`
	Agency          string `json:"agency,omitempty"`
	BroadcastifyURL string `json:"broadcastify_url"`
}

// ServeScanners handles GET /api/v1/scanners/{area} from operator config.
func (s *Service) ServeScanners(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	areaID := strings.Trim(strings.TrimPrefix(r.URL.Path, ScannersPrefix), "/")
	area, ok := s.resolveArea(areaID)
	if !ok {
		http.Error(w, fmt.Sprintf("unknown hazard area: %q", areaID), http.StatusNotFound)
		return
	}
	out := s.scanners(area)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = json.NewEncoder(w).Encode(out)
}

// layerMetadata carries per-layer collection metadata + the fail-loud flag.
type layerMetadata struct {
	attribution      string
	sourceURL        string
	emptyUnavailable bool // empty result => UNAVAILABLE (active-events-only sources)
}

func layerMeta(layer string) layerMetadata {
	switch layer {
	case LayerEvacuation:
		return layerMetadata{
			attribution:      "Cal OES / California County Governments — reference only",
			sourceURL:        caloes.SourceURL,
			emptyUnavailable: true,
		}
	default:
		return layerMetadata{}
	}
}

// layerResult is the outcome of building one layer: its features plus the
// fail-loud-adjusted source status and metadata.
type layerResult struct {
	features []Feature
	status   string
	meta     layerMetadata
}

// buildLayer runs one layer's builder and applies the fail-loud rules uniformly
// (builder error => UNAVAILABLE; empty active-events-only source => UNAVAILABLE).
// Both the single-layer endpoint and the situation aggregator go through here so
// the "empty never means all-clear" semantics can't drift between them.
func (s *Service) buildLayer(ctx context.Context, area config.HazardArea, layer string, build builder) layerResult {
	status := "OK"
	features, err := build(ctx, area)
	if err != nil {
		logging.Errorw(ctx, "Hazard layer build failed", "layer", layer, "area", area.ID, "error", err)
		status = "UNAVAILABLE"
		features = nil
	}
	meta := layerMeta(layer)
	if meta.emptyUnavailable && status == "OK" && len(features) == 0 {
		status = "UNAVAILABLE"
	}
	return layerResult{features: features, status: status, meta: meta}
}

func (s *Service) resolveArea(id string) (config.HazardArea, bool) {
	for _, a := range s.cfg.Hazards.Areas {
		if a.ID == id {
			return a, true
		}
	}
	return config.HazardArea{}, false
}

// --- layer builders (re-project existing feeds) ---

func (s *Service) roadIncidents(ctx context.Context, area config.HazardArea) ([]Feature, error) {
	resp, err := s.roads.ListIncidents(ctx, &api.ListIncidentsRequest{Area: area.IncidentArea})
	if err != nil {
		return nil, err
	}
	var out []Feature
	for _, in := range resp.GetIncidents() {
		loc := in.GetLocation()
		if loc == nil {
			continue
		}
		p := Properties{
			ID:        "chp:" + in.GetId(),
			Layer:     LayerRoadIncident,
			Kind:      "Road incident",
			Category:  strings.ToLower(strings.TrimPrefix(in.GetType().String(), "ALERT_TYPE_")),
			Headline:  in.GetDescription(),
			Status:    strings.ToLower(strings.TrimPrefix(in.GetStatus().String(), "INCIDENT_STATUS_")),
			AreaLabel: in.GetLocationDescription(),
			Source:    Source{ID: "chp", Name: "CHP / Caltrans", Attribution: "quickmap.dot.ca.gov"},
			Incident:  &IncidentProps{LogNumber: in.GetLogNumber()},
			Effective: tsToRFC3339(in.GetStarted()),
			UpdatedAt: tsToRFC3339(in.GetLastUpdated()),
		}
		p.setSeverity(fromAlertSeverity(in.GetSeverity()))
		out = append(out, Feature{Type: "Feature", Geometry: PointGeom(loc.GetLatitude(), loc.GetLongitude()), Properties: p})
	}
	return out, nil
}

func (s *Service) chainControls(ctx context.Context, area config.HazardArea) ([]Feature, error) {
	controls, err := s.caltrans.ParseChainControlsDetailed(ctx)
	if err != nil {
		return nil, err
	}
	var out []Feature
	for _, c := range controls {
		if c.Coordinates == nil || !area.Bounds.Contains(c.Coordinates.Latitude, c.Coordinates.Longitude) {
			continue
		}
		p := Properties{
			ID:           "cc:" + nonEmpty(c.MessageID, c.LocationName),
			Layer:        LayerChainControl,
			Kind:         "Chain control",
			Category:     strings.ToLower(c.Level),
			Headline:     strings.TrimSpace(c.Highway + " chain control " + c.Level),
			Description:  c.Description,
			AreaLabel:    c.LocationName,
			Effective:    c.EffectiveTime,
			Source:       Source{ID: "caltrans", Name: "Caltrans", Attribution: "quickmap.dot.ca.gov"},
			ChainControl: &ChainControlProps{Level: c.Level, Highway: c.Highway, Direction: c.Direction},
		}
		p.setSeverity(fromChainLevelStr(c.Level))
		out = append(out, Feature{Type: "Feature", Geometry: PointGeom(c.Coordinates.Latitude, c.Coordinates.Longitude), Properties: p})
	}
	return out, nil
}

func (s *Service) roadSegments(ctx context.Context, area config.HazardArea) ([]Feature, error) {
	resp, err := s.roads.ListRoads(ctx, &api.ListRoadsRequest{})
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*api.Road, len(resp.GetRoads()))
	for _, rd := range resp.GetRoads() {
		byID[rd.GetId()] = rd
	}

	var out []Feature
	for _, mr := range s.cfg.Roads.MonitoredRoads {
		// Include the segment if either endpoint is in the area.
		if !area.Bounds.Contains(mr.Origin.Latitude, mr.Origin.Longitude) &&
			!area.Bounds.Contains(mr.Destination.Latitude, mr.Destination.Longitude) {
			continue
		}
		geom := LineStringGeom([]LatLng{
			{Lat: mr.Origin.Latitude, Lng: mr.Origin.Longitude},
			{Lat: mr.Destination.Latitude, Lng: mr.Destination.Longitude},
		})
		p := Properties{
			ID:        "road:" + mr.ID,
			Layer:     LayerRoadSegment,
			Kind:      "Road segment",
			Headline:  strings.TrimSpace(mr.Name + " — " + mr.Section),
			AreaLabel: mr.Section,
			Source:    Source{ID: "google", Name: "Google Routes + Caltrans"},
			Road:      &RoadProps{RoadID: mr.ID},
		}
		sev := SevInfo
		if rd := byID[mr.ID]; rd != nil {
			p.Status = strings.ToLower(strings.TrimPrefix(rd.GetStatus().String(), "ROAD_STATUS_"))
			p.Road.Congestion = strings.TrimPrefix(rd.GetCongestionLevel().String(), "CONGESTION_LEVEL_")
			p.Road.DelayMinutes = rd.GetDelayMinutes()
			p.Road.DurationMinutes = rd.GetDurationMinutes()
			p.Road.DistanceKm = rd.GetDistanceKm()
			sev = roadSeverity(rd)
			if e := rd.GetStatusExplanation(); e != "" {
				p.Description = e
			}
		}
		p.setSeverity(sev)
		out = append(out, Feature{Type: "Feature", Geometry: geom, Properties: p})
	}
	return out, nil
}

func (s *Service) weatherAlerts(ctx context.Context, _ config.HazardArea) ([]Feature, error) {
	resp, err := s.weather.ListWeatherAlerts(ctx, &api.ListWeatherAlertsRequest{})
	if err != nil {
		return nil, err
	}
	var out []Feature
	for _, a := range resp.GetAlerts() {
		// M1: NWS zone polygons aren't fetched yet, so these are null-geometry
		// banner features (valid per the model).
		p := Properties{
			ID:          "wx:" + a.GetId(),
			Layer:       LayerWeatherAlert,
			Kind:        "Weather alert",
			Category:    a.GetEvent(),
			Headline:    nonEmpty(a.GetHeadline(), a.GetEvent()),
			Description: a.GetDescription(),
			Effective:   tsToRFC3339(a.GetStartTime()),
			Expires:     tsToRFC3339(a.GetEndTime()),
			Source:      Source{ID: strings.ToLower(a.GetSource().String()), Name: a.GetSenderName()},
			Weather: &WeatherProps{
				Event:  a.GetEvent(),
				Source: a.GetSource().String(),
				Zones:  a.GetZones(),
			},
		}
		p.setSeverity(fromAlertSeverity(a.GetSeverity()))
		out = append(out, Feature{Type: "Feature", Geometry: nil, Properties: p})
	}
	return out, nil
}

func (s *Service) fireWeather(ctx context.Context, _ config.HazardArea) ([]Feature, error) {
	resp, err := s.weather.ListWeather(ctx, &api.ListWeatherRequest{})
	if err != nil {
		return nil, err
	}
	fw := resp.GetFireWeather()
	if fw == nil {
		return nil, nil
	}
	state := strings.ToLower(strings.TrimPrefix(fw.GetState().String(), "FIRE_WEATHER_STATE_"))
	state = strings.ReplaceAll(state, "_", "-")
	p := Properties{
		ID:        "fw:region",
		Layer:     LayerFireWeather,
		Kind:      "Fire weather",
		Category:  state,
		Headline:  nonEmpty(fw.GetHeadline(), "Fire weather: "+state),
		Effective: tsToRFC3339(fw.GetEffective()),
		Expires:   tsToRFC3339(fw.GetExpires()),
		Source:    Source{ID: "nws", Name: nonEmpty(fw.GetSenderName(), "National Weather Service")},
		FireWeather: &FireWeatherProps{
			State:       state,
			SourceEvent: fw.GetSourceEvent(),
			Zones:       fw.GetZones(),
		},
	}
	p.setSeverity(fromFireWeatherState(state))
	// Region-wide, so null geometry (banner).
	return []Feature{{Type: "Feature", Geometry: nil, Properties: p}}, nil
}

func (s *Service) earthquakes(ctx context.Context, area config.HazardArea) ([]Feature, error) {
	quakes, err := s.usgs.GetEarthquakes(ctx, usgs.Bounds{
		MinLatitude:  area.Bounds.MinLatitude,
		MaxLatitude:  area.Bounds.MaxLatitude,
		MinLongitude: area.Bounds.MinLongitude,
		MaxLongitude: area.Bounds.MaxLongitude,
	}, 2.5, 7*24*time.Hour)
	if err != nil {
		return nil, err
	}
	var out []Feature
	for _, q := range quakes {
		p := Properties{
			ID:        "usgs:" + q.ID,
			Layer:     LayerEarthquake,
			Kind:      "Earthquake",
			Category:  "earthquake",
			Headline:  fmt.Sprintf("M%.1f — %s", q.Magnitude, q.Place),
			AreaLabel: q.Place,
			Source:    Source{ID: "usgs", Name: "USGS", URL: safeURL(q.URL), Attribution: "U.S. Geological Survey"},
			Earthquake: &EarthquakeProps{
				Magnitude: q.Magnitude,
				DepthKm:   q.DepthKm,
				Felt:      q.Felt,
			},
		}
		if !q.Time.IsZero() {
			p.Effective = q.Time.Format(time.RFC3339)
		}
		if !q.Updated.IsZero() {
			p.UpdatedAt = q.Updated.Format(time.RFC3339)
		}
		p.setSeverity(fromMagnitude(q.Magnitude))
		out = append(out, Feature{Type: "Feature", Geometry: PointGeom(q.Lat, q.Lng), Properties: p})
	}
	return out, nil
}

func (s *Service) wildfires(ctx context.Context, area config.HazardArea) ([]Feature, error) {
	bounds := wfigs.Bounds{
		MinLatitude:  area.Bounds.MinLatitude,
		MaxLatitude:  area.Bounds.MaxLatitude,
		MinLongitude: area.Bounds.MinLongitude,
		MaxLongitude: area.Bounds.MaxLongitude,
	}
	// Fetch the two independent sources concurrently — sequential fetches stack
	// their timeouts (up to ~45s) inside the /situation fan-out.
	var (
		incidents  []calfire.Incident
		perims     []wfigs.Perimeter
		ierr, perr error
		wg         sync.WaitGroup
	)
	wg.Add(2)
	go func() { defer wg.Done(); incidents, ierr = s.calfire.GetActiveIncidents(ctx) }()
	go func() { defer wg.Done(); perims, perr = s.wfigs.GetPerimeters(ctx, bounds) }()
	wg.Wait()

	if ierr != nil && perr != nil {
		return nil, fmt.Errorf("both wildfire sources failed: calfire=%v wfigs=%v", ierr, perr)
	}
	// Single-source failure still returns partial data as OK, but log it — a
	// silent drop of one source during a fire would otherwise be invisible.
	if ierr != nil {
		logging.Warnw(ctx, "CAL FIRE incident source failed; wildfire layer is partial (WFIGS perimeters only)", "error", ierr)
	}
	if perr != nil {
		logging.Warnw(ctx, "WFIGS perimeter source failed; wildfire layer is partial (CAL FIRE incidents only)", "error", perr)
	}

	// Index perimeters by normalized name so a CAL FIRE incident can adopt its
	// polygon geometry (join on incident name).
	byName := make(map[string]wfigs.Perimeter, len(perims))
	for _, p := range perims {
		byName[normFireName(p.Name)] = p
	}
	used := make(map[string]bool)

	var out []Feature
	for _, in := range incidents {
		if in.Lat == 0 && in.Lng == 0 {
			continue
		}
		if !area.Bounds.Contains(in.Lat, in.Lng) {
			continue
		}
		wf := &WildfireProps{
			Acres:       in.Acres,
			Containment: in.PercentContained,
			County:      in.County,
		}
		p := Properties{
			ID:        "calfire:" + nonEmpty(in.UniqueID, normFireName(in.Name)),
			Layer:     LayerWildfire,
			Kind:      "Wildfire",
			Category:  "wildfire",
			Headline:  fmt.Sprintf("%s — %.0f ac, %d%% contained", in.Name, in.Acres, in.PercentContained),
			AreaLabel: nonEmpty(in.Location, in.County),
			Status:    "active",
			Effective: tsOrEmpty(in.Started),
			UpdatedAt: tsOrEmpty(in.Updated),
			Source:    Source{ID: "calfire", Name: "CAL FIRE", URL: safeURL(in.URL), Attribution: "CAL FIRE / WFIGS"},
			Wildfire:  wf,
		}
		p.setSeverity(fromWildfire(in.PercentContained))
		// Adopt the matching perimeter polygon if we have one; else a point.
		if perim, ok := byName[normFireName(in.Name)]; ok {
			used[normFireName(in.Name)] = true
			wf.HasPerimeter = true
			out = append(out, Feature{Type: "Feature", Geometry: RawGeom(perim.GeometryType, perim.GeometryCoords), Properties: p})
		} else {
			out = append(out, Feature{Type: "Feature", Geometry: PointGeom(in.Lat, in.Lng), Properties: p})
		}
	}

	// Emit perimeters that didn't match a CAL FIRE incident as standalone
	// polygons (don't drop mapped fires CAL FIRE's curated list omits).
	for _, perim := range perims {
		if used[normFireName(perim.Name)] || perim.GeometryType == "" {
			continue
		}
		p := Properties{
			ID:       "wfigs:" + normFireName(perim.Name),
			Layer:    LayerWildfire,
			Kind:     "Wildfire",
			Category: "wildfire",
			Headline: fmt.Sprintf("%s — %.0f ac, %d%% contained", perim.Name, perim.Acres, perim.PercentContained),
			Status:   "active",
			Source:   Source{ID: "wfigs", Name: "NIFC WFIGS", Attribution: "NIFC / WFIGS"},
			Wildfire: &WildfireProps{Acres: perim.Acres, Containment: perim.PercentContained, Cause: perim.Cause, HasPerimeter: true},
		}
		p.setSeverity(fromWildfire(perim.PercentContained))
		out = append(out, Feature{Type: "Feature", Geometry: RawGeom(perim.GeometryType, perim.GeometryCoords), Properties: p})
	}
	return out, nil
}

func (s *Service) evacuations(ctx context.Context, area config.HazardArea) ([]Feature, error) {
	zones, err := s.caloes.GetActiveEvacuations(ctx, area.EvacCounties)
	if err != nil {
		return nil, err
	}
	var out []Feature
	for _, z := range zones {
		level := normalizeEvacLevel(z.Status)
		if level == "" {
			continue // only surface active Order/Warning/Advisory/SIP zones
		}
		if !evacStatusRecognized(z.Status) {
			// Conservatively classified as WARNING by the fail-loud default. Log so
			// the phrasing can be added to normalizeEvacLevel explicitly.
			logging.Warnw(ctx, "Unrecognized Cal OES evacuation STATUS; defaulted to WARNING",
				"status", z.Status, "zone", nonEmpty(z.ZoneID, z.ZoneName), "county", z.County)
		}
		human := titleCase(strings.ToLower(strings.ReplaceAll(level, "_", " ")))
		p := Properties{
			ID:          "evac:" + nonEmpty(z.ZoneID, z.ZoneName),
			Layer:       LayerEvacuation,
			Kind:        "Evacuation",
			Category:    strings.ToLower(level),
			Headline:    fmt.Sprintf("Evacuation %s — %s", human, nonEmpty(z.ZoneName, z.County)),
			Description: z.PublicInfo,
			Status:      level,
			AreaLabel:   nonEmpty(z.ZoneName, z.County),
			UpdatedAt:   tsOrEmpty(z.LastUpdated),
			Source:      Source{ID: "caloes", Name: "Cal OES", URL: caloes.SourceURL, Attribution: "Cal OES — reference only"},
			Evacuation:  &EvacuationProps{ZoneID: z.ZoneID, Level: level, EventType: z.EventType, County: z.County},
		}
		p.setSeverity(fromEvacLevel(level))
		out = append(out, Feature{Type: "Feature", Geometry: RawGeom(z.GeometryType, z.GeometryCoords), Properties: p})
	}
	return out, nil
}

// normFireName normalizes an incident/perimeter name for joining CAL FIRE and
// WFIGS (e.g. "Salt Springs Fire" and "Salt Springs" → "saltsprings").
func normFireName(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSuffix(strings.TrimSpace(s), " fire")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// roadSeverity derives a unified severity from a road's status + congestion.
func roadSeverity(rd *api.Road) string {
	switch rd.GetStatus() {
	case api.RoadStatus_CLOSED:
		return SevSevere
	case api.RoadStatus_RESTRICTED, api.RoadStatus_MAINTENANCE:
		return SevModerate
	}
	switch rd.GetCongestionLevel() {
	case api.CongestionLevel_SEVERE, api.CongestionLevel_HEAVY:
		return SevModerate
	case api.CongestionLevel_MODERATE:
		return SevMinor
	default:
		return SevInfo
	}
}

func tsToRFC3339(ts interface{ GetSeconds() int64 }) string {
	// Accept any *timestamppb.Timestamp via its getter; nil-safe.
	if ts == nil {
		return ""
	}
	secs := ts.GetSeconds()
	if secs == 0 {
		return ""
	}
	return time.Unix(secs, 0).UTC().Format(time.RFC3339)
}

// tsOrEmpty formats a time.Time as RFC3339, or "" if zero.
func tsOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// titleCase upper-cases the first letter of each space-separated word (ASCII).
// Replaces the deprecated strings.Title; inputs are known evac-level words.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func nonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
