package caltrans

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/lib/geo"
)

// CaltransFeedType represents the type of Caltrans feed
type CaltransFeedType int

const (
	CHAIN_CONTROL CaltransFeedType = iota
	LANE_CLOSURE
	CHP_INCIDENT
)

// HTTPDoer interface for HTTP clients (for testability)
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// FeedParser processes Caltrans KML feeds
// Implementation per research.md lines 49-67
type FeedParser struct {
	HTTPClient HTTPDoer
	geoUtils   geo.GeoUtils
}

// CaltransIncident represents parsed incident data from KML feeds
// Structure per data-model.md lines 66-78
type CaltransIncident struct {
	FeedType        CaltransFeedType
	Name            string
	DescriptionHtml string
	DescriptionText string
	StyleUrl        string
	Coordinates     *api.Coordinates  // Point location (for incidents)
	AffectedArea    *api.Polyline     // Polyline/polygon for closures
	ParsedStatus    string
	ParsedDates     []string
	LastFetched     time.Time
}

// KML XML structures for parsing
type KML struct {
	XMLName  xml.Name `xml:"kml"`
	Document Document `xml:"Document"`
}

type Document struct {
	XMLName    xml.Name    `xml:"Document"`
	Name       string      `xml:"name"`
	Placemarks []Placemark `xml:"Placemark"`
	Folders    []Folder    `xml:"Folder"`
}

type Folder struct {
	XMLName    xml.Name    `xml:"Folder"`
	Name       string      `xml:"name"`
	Placemarks []Placemark `xml:"Placemark"`
}

type Placemark struct {
	XMLName      xml.Name      `xml:"Placemark"`
	Name         string        `xml:"name"`
	Description  string        `xml:"description"`
	StyleURL     string        `xml:"styleUrl"`
	Point        Point         `xml:"Point"`
	LineString   LineString    `xml:"LineString"`
	Polygon      Polygon       `xml:"Polygon"`
	MultiGeometry MultiGeometry `xml:"MultiGeometry"`
}

type Point struct {
	XMLName     xml.Name `xml:"Point"`
	Coordinates string   `xml:"coordinates"`
}

type LineString struct {
	XMLName     xml.Name `xml:"LineString"`
	Coordinates string   `xml:"coordinates"`
}

type Polygon struct {
	XMLName        xml.Name `xml:"Polygon"`
	OuterBoundary  OuterBoundary `xml:"outerBoundaryIs"`
	InnerBoundary  []InnerBoundary `xml:"innerBoundaryIs"`
}

type OuterBoundary struct {
	XMLName    xml.Name   `xml:"outerBoundaryIs"`
	LinearRing LinearRing `xml:"LinearRing"`
}

type InnerBoundary struct {
	XMLName    xml.Name   `xml:"innerBoundaryIs"`
	LinearRing LinearRing `xml:"LinearRing"`
}

type LinearRing struct {
	XMLName     xml.Name `xml:"LinearRing"`
	Coordinates string   `xml:"coordinates"`
}

type MultiGeometry struct {
	XMLName     xml.Name     `xml:"MultiGeometry"`
	Points      []Point      `xml:"Point"`
	LineStrings []LineString `xml:"LineString"`
	Polygons    []Polygon    `xml:"Polygon"`
}

// NewFeedParser creates a new Caltrans KML feed parser
func NewFeedParser() *FeedParser {
	return &FeedParser{
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		geoUtils: geo.NewGeoUtils(),
	}
}

// ParseChainControls processes chain control KML feed
// URL from research.md line 71
func (p *FeedParser) ParseChainControls(ctx context.Context) ([]CaltransIncident, error) {
	return p.parseKMLFeed(ctx, "https://quickmap.dot.ca.gov/data/cc.kml", CHAIN_CONTROL)
}

// ParseLaneClosures processes lane closures KML feed  
// URL from research.md line 72
func (p *FeedParser) ParseLaneClosures(ctx context.Context) ([]CaltransIncident, error) {
	return p.parseKMLFeed(ctx, "https://quickmap.dot.ca.gov/data/lcs2way.kml", LANE_CLOSURE)
}

// ParseCHPIncidents processes CHP incidents KML feed
// URL from research.md line 73
func (p *FeedParser) ParseCHPIncidents(ctx context.Context) ([]CaltransIncident, error) {
	return p.parseKMLFeed(ctx, "https://quickmap.dot.ca.gov/data/chp-only.kml", CHP_INCIDENT)
}

// ParseWithGeographicFilter parses incidents and filters by proximity to route coordinates
// Implementation per research.md line 79
func (p *FeedParser) ParseWithGeographicFilter(ctx context.Context, routeCoordinates []geo.Point, radiusMeters float64) ([]CaltransIncident, error) {
	// Parse feeds (chain control parsing disabled until winter data available)
	// TODO: Re-enable chain control parsing in winter when actual chain requirement data is available
	
	laneClosures, err := p.ParseLaneClosures(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lane closures: %w", err)
	}

	chpIncidents, err := p.ParseCHPIncidents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CHP incidents: %w", err)
	}

	// Combine incidents (excluding chain controls for now)
	allIncidents := append(laneClosures, chpIncidents...)

	// Filter by geographic proximity using centralized filtering
	filteredIncidents := make([]CaltransIncident, 0)
	
	for _, incident := range allIncidents {
		// Skip incidents without valid coordinates
		if incident.Coordinates == nil {
			continue
		}
		
		incidentPoint := geo.Point{
			Latitude:  incident.Coordinates.Latitude,
			Longitude: incident.Coordinates.Longitude,
		}
		
		// Check if incident is within radius of any route coordinate
		isNearRoute := false
		for _, coord := range routeCoordinates {
			distance, err := p.geoUtils.DistanceFromCoords(
				coord.Latitude, coord.Longitude,
				incidentPoint.Latitude, incidentPoint.Longitude,
			)
			if err != nil {
				continue // Skip invalid coordinates
			}
			if distance <= radiusMeters {
				isNearRoute = true
				break // Found within range, no need to check other coordinates
			}
		}
		
		if isNearRoute {
			filteredIncidents = append(filteredIncidents, incident)
		}
	}

	return filteredIncidents, nil
}

// parseKMLFeed downloads and parses a KML feed
func (p *FeedParser) parseKMLFeed(ctx context.Context, url string, feedType CaltransFeedType) ([]CaltransIncident, error) {
	// Download KML file
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Default to a new HTTP client if none is set
	httpClient := p.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download KML: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error %d downloading KML from %s", resp.StatusCode, url)
	}

	// Parse KML using standard encoding/xml
	kmlData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read KML response: %w", err)
	}

	var kml KML
	err = xml.Unmarshal(kmlData, &kml)
	if err != nil {
		return nil, fmt.Errorf("failed to parse KML: %w", err)
	}

	// Process KML placemarks
	var incidents []CaltransIncident
	now := time.Now()

	// Process placemarks directly in document
	for _, placemark := range kml.Document.Placemarks {
		if incident := p.processPlacemark(&placemark, feedType, now); incident != nil {
			incidents = append(incidents, *incident)
		}
	}

	// Process placemarks in folders
	for _, folder := range kml.Document.Folders {
		for _, placemark := range folder.Placemarks {
			if incident := p.processPlacemark(&placemark, feedType, now); incident != nil {
				incidents = append(incidents, *incident)
			}
		}
	}

	return incidents, nil
}

// processPlacemark converts KML Placemark to CaltransIncident
// Structure mapping per data-model.md lines 80-90
func (p *FeedParser) processPlacemark(placemark *Placemark, feedType CaltransFeedType, fetchTime time.Time) *CaltransIncident {
	// Extract geometry data (coordinates and polylines)
	coordinates, polyline := p.extractGeometry(placemark)
	
	// Skip placemarks with no valid geometry
	if coordinates == nil && polyline == nil {
		return nil
	}

	// Extract description HTML
	descriptionHtml := placemark.Description

	// Extract plain text from HTML description
	descriptionText := extractTextFromHTML(descriptionHtml)

	// Extract status and dates from description
	parsedStatus := extractStatus(descriptionText)
	parsedDates := extractDates(descriptionText)

	return &CaltransIncident{
		FeedType:        feedType,
		Name:            placemark.Name,
		DescriptionHtml: descriptionHtml,
		DescriptionText: descriptionText,
		StyleUrl:        placemark.StyleURL,
		Coordinates:     coordinates,
		AffectedArea:    polyline,
		ParsedStatus:    parsedStatus,
		ParsedDates:     parsedDates,
		LastFetched:     fetchTime,
	}
}

// extractGeometry extracts coordinate and polyline data from a placemark
func (p *FeedParser) extractGeometry(placemark *Placemark) (*api.Coordinates, *api.Polyline) {
	var pointCoord *api.Coordinates
	var polyline *api.Polyline

	// Handle Point geometry
	if placemark.Point.Coordinates != "" {
		pointCoord = p.parseCoordinates(placemark.Point.Coordinates)
	}

	// Handle LineString geometry (for linear closures)
	if placemark.LineString.Coordinates != "" {
		coords := p.parseCoordinateList(placemark.LineString.Coordinates)
		if len(coords) > 0 {
			polyline = &api.Polyline{Points: coords}
			// Use first point as primary coordinate if no point geometry
			if pointCoord == nil && len(coords) > 0 {
				pointCoord = coords[0]
			}
		}
	}

	// Handle Polygon geometry (for area closures)
	if placemark.Polygon.OuterBoundary.LinearRing.Coordinates != "" {
		coords := p.parseCoordinateList(placemark.Polygon.OuterBoundary.LinearRing.Coordinates)
		if len(coords) > 0 {
			polyline = &api.Polyline{Points: coords}
			// Use first point as primary coordinate if no point geometry
			if pointCoord == nil && len(coords) > 0 {
				pointCoord = coords[0]
			}
		}
	}

	// Handle MultiGeometry (complex geometries)
	if len(placemark.MultiGeometry.Points) > 0 || 
	   len(placemark.MultiGeometry.LineStrings) > 0 || 
	   len(placemark.MultiGeometry.Polygons) > 0 {
		
		var allCoords []*api.Coordinates

		// Collect all points from MultiGeometry
		for _, point := range placemark.MultiGeometry.Points {
			if coord := p.parseCoordinates(point.Coordinates); coord != nil {
				allCoords = append(allCoords, coord)
				if pointCoord == nil {
					pointCoord = coord
				}
			}
		}

		// Collect all coordinates from LineStrings
		for _, lineString := range placemark.MultiGeometry.LineStrings {
			coords := p.parseCoordinateList(lineString.Coordinates)
			allCoords = append(allCoords, coords...)
			if pointCoord == nil && len(coords) > 0 {
				pointCoord = coords[0]
			}
		}

		// Collect all coordinates from Polygons
		for _, polygon := range placemark.MultiGeometry.Polygons {
			coords := p.parseCoordinateList(polygon.OuterBoundary.LinearRing.Coordinates)
			allCoords = append(allCoords, coords...)
			if pointCoord == nil && len(coords) > 0 {
				pointCoord = coords[0]
			}
		}

		// Create polyline from all collected coordinates
		if len(allCoords) > 1 {
			polyline = &api.Polyline{Points: allCoords}
		}
	}

	return pointCoord, polyline
}

// parseCoordinates parses KML coordinate string "longitude,latitude,altitude"
func (p *FeedParser) parseCoordinates(coordString string) *api.Coordinates {
	coordString = strings.TrimSpace(coordString)
	if coordString == "" {
		return nil
	}

	// KML coordinates format: "longitude,latitude,altitude"
	parts := strings.Split(coordString, ",")
	if len(parts) < 2 {
		return nil
	}

	longitude, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return nil
	}

	latitude, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return nil
	}

	return &api.Coordinates{
		Latitude:  latitude,
		Longitude: longitude,
	}
}

// parseCoordinateList parses KML coordinate string with multiple coordinates
// Format: "lon1,lat1,alt1 lon2,lat2,alt2 lon3,lat3,alt3"
func (p *FeedParser) parseCoordinateList(coordString string) []*api.Coordinates {
	coordString = strings.TrimSpace(coordString)
	if coordString == "" {
		return nil
	}

	var coordinates []*api.Coordinates

	// Split by whitespace or newlines to get individual coordinate sets
	coordSets := regexp.MustCompile(`\s+`).Split(coordString, -1)
	
	for _, coordSet := range coordSets {
		coordSet = strings.TrimSpace(coordSet)
		if coordSet == "" {
			continue
		}

		// Parse individual coordinate set "longitude,latitude,altitude"
		parts := strings.Split(coordSet, ",")
		if len(parts) < 2 {
			continue
		}

		longitude, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil {
			continue
		}

		latitude, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			continue
		}

		coordinates = append(coordinates, &api.Coordinates{
			Latitude:  latitude,
			Longitude: longitude,
		})
	}

	return coordinates
}

// extractTextFromHTML removes HTML tags and decodes HTML entities
func extractTextFromHTML(htmlContent string) string {
	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(htmlContent, " ")

	// Decode HTML entities
	text = html.UnescapeString(text)

	// Clean up whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// extractStatus attempts to extract status information from description text
func extractStatus(text string) string {
	// Common status patterns in Caltrans descriptions
	statusPatterns := []string{
		`(?i)(closed?)`,
		`(?i)(chain control in effect)`,
		`(?i)(restrictions?)`,
		`(?i)(incident)`,
		`(?i)(construction)`,
	}

	for _, pattern := range statusPatterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindString(text); match != "" {
			return strings.ToLower(match)
		}
	}

	return ""
}

// extractDates attempts to extract date/time information from description text
func extractDates(text string) []string {
	// Pattern for dates like "12/25/2024" or "Dec 25, 2024"
	datePattern := regexp.MustCompile(`\d{1,2}[/\-]\d{1,2}[/\-]\d{4}|[A-Za-z]{3}\s+\d{1,2},\s+\d{4}`)
	matches := datePattern.FindAllString(text, -1)

	// Deduplicate dates
	seen := make(map[string]bool)
	var uniqueDates []string
	for _, date := range matches {
		if !seen[date] {
			seen[date] = true
			uniqueDates = append(uniqueDates, date)
		}
	}

	// Return empty slice instead of nil for consistency
	if uniqueDates == nil {
		return []string{}
	}
	return uniqueDates
}

