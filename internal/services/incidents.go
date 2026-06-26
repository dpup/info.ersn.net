package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dpup/prefab/logging"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/clients/caltrans"
	"github.com/dpup/info.ersn.net/server/internal/config"
)

// ListIncidents returns region-wide CHP/Caltrans dispatch incidents for a
// configured area (issue #7). Unlike the alerts embedded in each Road, this is
// a flat list scoped only by geography, with no per-route classification or AI
// enhancement - it is intentionally lightweight so the whole region can be
// surfaced cheaply.
func (s *RoadsService) ListIncidents(ctx context.Context, req *api.ListIncidentsRequest) (*api.ListIncidentsResponse, error) {
	logging.Infow(ctx, "ListIncidents called", "area", req.Area)

	area, ok := s.resolveIncidentArea(req.Area)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unknown incident area: %q", req.Area)
	}

	cacheKey := fmt.Sprintf("incidents:%s", area.ID)

	// Serve cached data when fresh; the underlying KML feeds change on the order
	// of minutes and are shared with the roads refresh.
	var cachedIncidents []*api.Incident
	entry, found, err := s.cache.GetWithMetadata(cacheKey, &cachedIncidents)
	if err != nil {
		logging.Errorw(ctx, "Cache error", "error", err, "cache_key", cacheKey)
	}
	if found && !s.cache.IsStale(cacheKey) {
		var lastUpdated *timestamppb.Timestamp
		if entry != nil {
			lastUpdated = timestamppb.New(entry.CreatedAt)
		}
		return &api.ListIncidentsResponse{
			Incidents:   cachedIncidents,
			LastUpdated: lastUpdated,
			Area:        area.ID,
		}, nil
	}

	incidents, err := s.refreshIncidents(ctx, area)
	if err != nil {
		// Fall back to stale cache rather than erroring if we have anything.
		if found {
			logging.Errorw(ctx, "Incident refresh failed, returning stale cache", "error", err)
			var lastUpdated *timestamppb.Timestamp
			if entry != nil {
				lastUpdated = timestamppb.New(entry.CreatedAt)
			}
			return &api.ListIncidentsResponse{
				Incidents:   cachedIncidents,
				LastUpdated: lastUpdated,
				Area:        area.ID,
			}, nil
		}
		return nil, fmt.Errorf("failed to refresh incidents: %w", err)
	}

	if err := s.cache.Set(cacheKey, incidents, s.config.Roads.CaltransFeeds.CHPIncidents.RefreshInterval, "incidents"); err != nil {
		logging.Errorw(ctx, "Failed to cache incidents", "error", err)
	}

	return &api.ListIncidentsResponse{
		Incidents:   incidents,
		LastUpdated: timestamppb.Now(),
		Area:        area.ID,
	}, nil
}

// resolveIncidentArea looks up an area by id. An empty id resolves to the first
// configured area for convenience.
func (s *RoadsService) resolveIncidentArea(id string) (config.IncidentArea, bool) {
	areas := s.config.Roads.IncidentAreas
	if len(areas) == 0 {
		return config.IncidentArea{}, false
	}
	if id == "" {
		return areas[0], true
	}
	for _, a := range areas {
		if a.ID == id {
			return a, true
		}
	}
	return config.IncidentArea{}, false
}

// refreshIncidents fetches CHP and lane-closure feeds and converts the ones
// inside the area bounds into structured incidents.
func (s *RoadsService) refreshIncidents(ctx context.Context, area config.IncidentArea) ([]*api.Incident, error) {
	chpIncidents, chpErr := s.caltransClient.ParseCHPIncidents(ctx)
	laneClosures, lcErr := s.caltransClient.ParseLaneClosures(ctx)
	if chpErr != nil && lcErr != nil {
		return nil, fmt.Errorf("both incident feeds failed: chp=%v lanes=%v", chpErr, lcErr)
	}

	incidents := s.normalizeIncidents(area, chpIncidents, laneClosures)

	logging.Infow(ctx, "Region-wide incidents refreshed",
		"area", area.ID,
		"chp_total", len(chpIncidents),
		"lane_total", len(laneClosures),
		"in_area", len(incidents))

	return incidents, nil
}

// normalizeIncidents builds a clean, one-entry-per-incident list from the raw
// feeds. It drops geometry-only placemarks and collapses duplicates: the
// Caltrans lane-closure feed emits a separate LineString "path" placemark per
// closure (no description) and repeats closures across directions, neither of
// which belongs in a flat list. CHP incidents come first, then lane closures.
func (s *RoadsService) normalizeIncidents(area config.IncidentArea, lists ...[]caltrans.CaltransIncident) []*api.Incident {
	var incidents []*api.Incident
	seen := make(map[string]bool)
	for _, list := range lists {
		for _, in := range list {
			inc := s.buildIncident(in, area)
			if inc == nil || inc.Description == "" {
				continue // outside bounds, no coordinates, or a geometry-only placemark
			}
			if inc.Id != "" {
				if seen[inc.Id] {
					continue
				}
				seen[inc.Id] = true
			}
			incidents = append(incidents, inc)
		}
	}
	return incidents
}

// buildIncident converts a Caltrans incident into the API representation,
// returning nil if it has no coordinates or falls outside the area bounds.
func (s *RoadsService) buildIncident(in caltrans.CaltransIncident, area config.IncidentArea) *api.Incident {
	if in.Coordinates == nil {
		return nil
	}
	if !area.Bounds.Contains(in.Coordinates.Latitude, in.Coordinates.Longitude) {
		return nil
	}

	d := parseIncidentDetail(in)

	description := d.title
	if description == "" {
		description = in.DescriptionText
	}
	locationDesc := d.location
	if locationDesc == "" {
		locationDesc = in.Name
	}

	inc := &api.Incident{
		Id:                  incidentID(in, d.logNumber),
		Type:                incidentType(in),
		Severity:            incidentSeverity(in, d.title),
		Location:            &api.Coordinates{Latitude: in.Coordinates.Latitude, Longitude: in.Coordinates.Longitude},
		LocationDescription: locationDesc,
		Description:         description,
		Status:              "active",
		LogNumber:           d.logNumber,
		Area:                area.ID,
	}
	if !d.started.IsZero() {
		inc.Started = timestamppb.New(d.started)
	}
	if !d.lastUpdated.IsZero() {
		inc.LastUpdated = timestamppb.New(d.lastUpdated)
	} else {
		inc.LastUpdated = timestamppb.New(in.LastFetched)
	}
	return inc
}

// incidentID builds a stable identifier, preferring the CHP log number.
func incidentID(in caltrans.CaltransIncident, logNumber string) string {
	if logNumber != "" {
		return logNumber
	}
	// Fall back to a slug of the name for lane closures without a log number.
	slug := strings.ToLower(strings.TrimSpace(in.Name))
	slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}

func incidentType(in caltrans.CaltransIncident) api.AlertType {
	switch in.FeedType {
	case caltrans.LANE_CLOSURE:
		return api.AlertType_CLOSURE
	case caltrans.CHP_INCIDENT:
		return api.AlertType_INCIDENT
	default:
		return api.AlertType_ALERT_TYPE_UNSPECIFIED
	}
}

// incidentSeverity is a lightweight, non-AI heuristic based on the incident text
// and feed type. The region-wide feed favours breadth over per-incident analysis.
func incidentSeverity(in caltrans.CaltransIncident, typeText string) api.AlertSeverity {
	lower := strings.ToLower(typeText + " " + in.DescriptionText + " " + in.StyleUrl)

	switch {
	case strings.Contains(lower, "full-closure"),
		strings.Contains(lower, "fatal"),
		strings.Contains(lower, "injury"),
		strings.Contains(lower, "fire"),
		strings.Contains(lower, "overturn"):
		return api.AlertSeverity_CRITICAL
	case in.FeedType == caltrans.LANE_CLOSURE,
		strings.Contains(lower, "collision"),
		strings.Contains(lower, "hazard"),
		strings.Contains(lower, "closure"):
		return api.AlertSeverity_WARNING
	case strings.Contains(lower, "assist"),
		strings.Contains(lower, "maintenance"),
		strings.Contains(lower, "traffic advisory"):
		return api.AlertSeverity_INFO
	default:
		return api.AlertSeverity_WARNING
	}
}

// incidentDetail holds the structured fields extracted from an incident's
// description markup.
type incidentDetail struct {
	logNumber   string
	title       string // incident type / headline text
	location    string // human-readable location
	started     time.Time
	lastUpdated time.Time
}

var (
	// 2026 "infowindow" markup.
	iwTitleRe     = regexp.MustCompile(`(?is)<h2[^>]*class="iw-title"[^>]*>(.*?)</h2>`)
	iwTextRe      = regexp.MustCompile(`(?is)<p[^>]*class="iw-text"[^>]*>(.*?)</p>`)
	chpLabelRe    = regexp.MustCompile(`(?i)CHP Incident\s+([A-Za-z0-9]+)`)
	logNumberRe   = regexp.MustCompile(`(?i)Log Number:\s*([A-Za-z0-9]+)`)
	closureIDRe   = regexp.MustCompile(`(?i)Closure ID:\s*([A-Za-z0-9]+)`)
	chpLogTokenRe = regexp.MustCompile(`([0-9]{6}[A-Z]{2}[0-9]{4})`)

	// Legacy (pre-2026) markup, kept for the older test fixtures.
	legacyParaRe = regexp.MustCompile(`(?is)<p[^>]*align="left"[^>]*>(.*?)</p>`)

	brRe          = regexp.MustCompile(`(?i)<br\s*/?>`)
	tagRe         = regexp.MustCompile(`<[^>]*>`)
	lastUpdatedRe = regexp.MustCompile(`(?i)Last updated:\s*(?:<strong>\s*)?([0-9]{1,2}/[0-9]{1,2}/[0-9]{4})(?:\s*</strong>)?\s*([0-9]{1,2}:[0-9]{2}[ap]m)`)
)

// parseIncidentDetail extracts structured fields from a Caltrans incident,
// handling both the 2026 iw-* markup and the legacy format.
func parseIncidentDetail(in caltrans.CaltransIncident) incidentDetail {
	html := in.DescriptionHtml
	d := incidentDetail{lastUpdated: parseLastUpdatedTime(html)}

	// Log number: CHP label, then explicit "Log Number" / "Closure ID".
	d.logNumber = extractLogNumber(in, html)

	// Title from iw-title (new) if present.
	if m := iwTitleRe.FindStringSubmatch(html); len(m) > 1 {
		d.title = cleanSegment(m[1])
	}

	if texts := iwTextRe.FindAllStringSubmatch(html, -1); len(texts) > 0 {
		// New format. CHP first text is "<time> <br> <location>"; lane closures
		// put the location/extent directly in the first text.
		segs := splitBR(texts[0][1])
		if in.FeedType == caltrans.CHP_INCIDENT && len(segs) > 0 {
			d.started = parseCHPTime(segs[0])
			if len(segs) > 1 {
				d.location = segs[1]
			}
		} else if len(segs) > 0 {
			d.location = strings.Join(segs, " ")
		}
	} else if m := legacyParaRe.FindStringSubmatch(html); len(m) > 1 {
		// Legacy format: "<time> <br> <type> <br> <location>".
		segs := splitBR(m[1])
		if len(segs) > 0 {
			d.started = parseCHPTime(segs[0])
		}
		if len(segs) > 1 && d.title == "" {
			d.title = segs[1]
		}
		if len(segs) > 2 {
			d.location = segs[2]
		}
	}

	return d
}

// extractLogNumber pulls the incident's identifier from its name or description.
func extractLogNumber(in caltrans.CaltransIncident, html string) string {
	// CHP log token in the name (e.g. "CHP Incident 250916ST0066").
	if m := chpLogTokenRe.FindString(in.Name); m != "" {
		return m
	}
	if m := chpLabelRe.FindStringSubmatch(in.Name); len(m) > 1 {
		return m[1]
	}
	if m := chpLabelRe.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	// Lane closure identifiers.
	if m := closureIDRe.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	if m := logNumberRe.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	return ""
}

// pacificTime is the timezone Caltrans/CHP feeds report times in. Times are
// parsed in this location so the resulting timestamps are accurate.
var pacificTime = mustLoadPacific()

func mustLoadPacific() *time.Location {
	if loc, err := time.LoadLocation("America/Los_Angeles"); err == nil {
		return loc
	}
	return time.UTC
}

func parseLastUpdatedTime(html string) time.Time {
	m := lastUpdatedRe.FindStringSubmatch(html)
	if len(m) < 3 {
		return time.Time{}
	}
	if t, err := time.ParseInLocation("1/2/2006 3:04pm", strings.TrimSpace(m[1]+" "+m[2]), pacificTime); err == nil {
		return t
	}
	return time.Time{}
}

// parseCHPTime parses CHP timestamps like "Jun 25 2026  6:24PM" (note the
// irregular double space and 12-hour clock), interpreted as Pacific time.
func parseCHPTime(s string) time.Time {
	s = strings.Join(strings.Fields(s), " ") // collapse whitespace
	for _, layout := range []string{"Jan 2 2006 3:04PM", "Jan 2 2006 3:04pm"} {
		if t, err := time.ParseInLocation(layout, s, pacificTime); err == nil {
			return t
		}
	}
	return time.Time{}
}

// splitBR splits an HTML fragment on <br> and returns cleaned, non-empty segments.
func splitBR(fragment string) []string {
	var segs []string
	for _, p := range brRe.Split(fragment, -1) {
		if clean := cleanSegment(p); clean != "" {
			segs = append(segs, clean)
		}
	}
	return segs
}

func cleanSegment(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(tagRe.ReplaceAllString(s, " ")), " "))
}
