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

	api "github.com/dpup/info.ersn.net/server"
)

// CaltransFeedType represents the type of Caltrans feed
type CaltransFeedType int

const (
	CHAIN_CONTROL CaltransFeedType = iota
	LANE_CLOSURE
	CHP_INCIDENT
)

// FeedParser processes Caltrans KML feeds
// Implementation per research.md lines 49-67
type FeedParser struct {
	HTTPClient *http.Client
}

// CaltransIncident represents parsed incident data from KML feeds
// Structure per data-model.md lines 66-78
type CaltransIncident struct {
	FeedType        CaltransFeedType
	Name            string
	DescriptionHtml string
	DescriptionText string
	StyleUrl        string
	Coordinates     *api.Coordinates
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
	XMLName     xml.Name `xml:"Placemark"`
	Name        string   `xml:"name"`
	Description string   `xml:"description"`
	StyleURL    string   `xml:"styleUrl"`
	Point       Point    `xml:"Point"`
}

type Point struct {
	XMLName     xml.Name `xml:"Point"`
	Coordinates string   `xml:"coordinates"`
}

// NewFeedParser creates a new Caltrans KML feed parser
func NewFeedParser() *FeedParser {
	return &FeedParser{
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
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
func (p *FeedParser) ParseWithGeographicFilter(ctx context.Context, routeCoordinates []struct{ Lat, Lon float64 }, radiusMeters float64) ([]CaltransIncident, error) {
	// Parse all feeds
	chainControls, err := p.ParseChainControls(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse chain controls: %w", err)
	}

	laneClosures, err := p.ParseLaneClosures(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lane closures: %w", err)
	}

	chpIncidents, err := p.ParseCHPIncidents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CHP incidents: %w", err)
	}

	// Combine all incidents
	allIncidents := append(chainControls, laneClosures...)
	allIncidents = append(allIncidents, chpIncidents...)

	// Filter by geographic proximity
	filteredIncidents := make([]CaltransIncident, 0)
	for _, incident := range allIncidents {
		for _, coord := range routeCoordinates {
			distance := haversineDistance(
				coord.Lat, coord.Lon,
				incident.Coordinates.Latitude, incident.Coordinates.Longitude,
			)
			if distance <= radiusMeters {
				filteredIncidents = append(filteredIncidents, incident)
				break // Found within range, no need to check other coordinates
			}
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
	defer resp.Body.Close()

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
	// Extract coordinates from Point geometry
	coordinates := p.parseCoordinates(placemark.Point.Coordinates)
	if coordinates == nil {
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
		ParsedStatus:    parsedStatus,
		ParsedDates:     parsedDates,
		LastFetched:     fetchTime,
	}
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

	return uniqueDates
}

// haversineDistance calculates the distance between two points on Earth in meters
func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusM = 6371000 // Earth's radius in meters

	// Convert degrees to radians
	lat1Rad := lat1 * (3.14159265359 / 180)
	lon1Rad := lon1 * (3.14159265359 / 180)
	lat2Rad := lat2 * (3.14159265359 / 180)
	lon2Rad := lon2 * (3.14159265359 / 180)

	// Haversine formula
	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad

	a := 0.5 - (dLat/2) + (lat1Rad)*(lat2Rad)*(0.5-(dLon/2))
	
	// Simplified haversine calculation
	return earthRadiusM * 2 * 3.14159265359 * a
}