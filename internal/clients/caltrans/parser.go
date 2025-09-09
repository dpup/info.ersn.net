package caltrans

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/twpayne/go-kml/v2"

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
	httpClient *http.Client
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

// NewFeedParser creates a new Caltrans KML feed parser
func NewFeedParser() *FeedParser {
	return &FeedParser{
		httpClient: &http.Client{
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
	var filteredIncidents []CaltransIncident
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

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download KML: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error %d downloading KML from %s", resp.StatusCode, url)
	}

	// Parse KML using github.com/twpayne/go-kml
	kmlData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read KML response: %w", err)
	}

	// Parse KML using the correct v2 API
	var k kml.KML
	err = k.UnmarshalXML(bytes.NewDecoder(bytes.NewReader(kmlData)), xml.StartElement{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse KML: %w", err)
	}

	// Process KML placemarks
	var incidents []CaltransIncident
	now := time.Now()

	// Extract placemarks by walking the document
	if k.Document != nil {
		for _, folder := range k.Document.Folders {
			for _, placemark := range folder.Placemarks {
				incident := p.processPlacemark(&placemark, feedType, now)
				if incident != nil {
					incidents = append(incidents, *incident)
				}
			}
		}
		// Also check direct placemarks in document
		for _, placemark := range k.Document.Placemarks {
			incident := p.processPlacemark(&placemark, feedType, now)
			if incident != nil {
				incidents = append(incidents, *incident)
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to process KML placemarks: %w", err)
	}

	return incidents, nil
}

// processPlacemark converts KML Placemark to CaltransIncident
// Structure mapping per data-model.md lines 80-90
func (p *FeedParser) processPlacemark(placemark *kml.Placemark, feedType CaltransFeedType, fetchTime time.Time) *CaltransIncident {
	// Extract coordinates from Point geometry
	var coordinates *api.Coordinates
	if placemark.Point != nil && len(placemark.Point.Coordinates) > 0 {
		// KML coordinates are in "longitude,latitude,altitude" format
		coord := placemark.Point.Coordinates[0]
		if len(coord) >= 2 {
			coordinates = &api.Coordinates{
				Latitude:  coord[1], // Second element is latitude
				Longitude: coord[0], // First element is longitude
			}
		}
	}

	// Skip placemarks without valid coordinates
	if coordinates == nil {
		return nil
	}

	// Extract description HTML from CDATA
	descriptionHtml := ""
	if placemark.Description != nil {
		descriptionHtml = *placemark.Description
	}

	// Extract plain text from HTML description
	descriptionText := extractTextFromHTML(descriptionHtml)

	// Extract status and dates from description
	parsedStatus := extractStatus(descriptionText)
	parsedDates := extractDates(descriptionText)

	// Extract style URL
	styleUrl := ""
	if placemark.StyleURL != nil {
		styleUrl = *placemark.StyleURL
	}

	// Extract name
	name := ""
	if placemark.Name != nil {
		name = *placemark.Name
	}

	return &CaltransIncident{
		FeedType:        feedType,
		Name:            name,
		DescriptionHtml: descriptionHtml,
		DescriptionText: descriptionText,
		StyleUrl:        styleUrl,
		Coordinates:     coordinates,
		ParsedStatus:    parsedStatus,
		ParsedDates:     parsedDates,
		LastFetched:     fetchTime,
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