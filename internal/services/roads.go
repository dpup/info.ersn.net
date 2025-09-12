package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/cache"
	"github.com/dpup/info.ersn.net/server/internal/clients/caltrans"
	"github.com/dpup/info.ersn.net/server/internal/clients/google"
	"github.com/dpup/info.ersn.net/server/internal/config"
	"github.com/dpup/info.ersn.net/server/internal/lib/alerts"
	"github.com/dpup/info.ersn.net/server/internal/lib/geo"
	"github.com/dpup/info.ersn.net/server/internal/lib/routing"
)

// RoadsService implements the gRPC RoadsService
// Implementation per tasks.md T016 and data-model.md Route entity
type RoadsService struct {
	api.UnimplementedRoadsServiceServer
	googleClient   *google.Client
	caltransClient *caltrans.FeedParser
	cache          *cache.Cache
	config         *config.RoadsConfig
	alertEnhancer  alerts.AlertEnhancer
	routeMatcher   routing.RouteMatcher
	geoUtils       geo.GeoUtils
}

// NewRoadsService creates a new RoadsService
func NewRoadsService(googleClient *google.Client, caltransClient *caltrans.FeedParser, cache *cache.Cache, config *config.RoadsConfig) *RoadsService {
	// Initialize alert enhancer if OpenAI is enabled and configured
	var alertEnhancer alerts.AlertEnhancer

	if config.OpenAI.Enabled && config.OpenAI.APIKey != "" {
		alertEnhancer = alerts.NewAlertEnhancer(config.OpenAI.APIKey, config.OpenAI.Model)
		log.Printf("OpenAI alert enhancement enabled with model: %s", config.OpenAI.Model)
	} else {
		log.Printf("OpenAI alert enhancement disabled")
	}

	return &RoadsService{
		googleClient:   googleClient,
		caltransClient: caltransClient,
		cache:          cache,
		config:         config,
		alertEnhancer:  alertEnhancer,
		routeMatcher:   routing.NewRouteMatcher(),
		geoUtils:       geo.NewGeoUtils(),
	}
}

// ListRoads implements the gRPC method defined in contracts/roads.proto line 12-17
func (s *RoadsService) ListRoads(ctx context.Context, req *api.ListRoadsRequest) (*api.ListRoadsResponse, error) {
	log.Printf("ListRoads called")

	// Try to get cached roads first
	var cachedRoads []*api.Road
	cacheKey := "roads:all"

	found, err := s.cache.Get(cacheKey, &cachedRoads)
	if err != nil {
		log.Printf("Cache error: %v", err)
	}

	if found && !s.cache.IsStale(cacheKey) {
		log.Printf("Returning cached roads (%d roads)", len(cachedRoads))

		// Get cache metadata for last_updated timestamp
		entry, _, _ := s.cache.GetWithMetadata(cacheKey, nil)
		var lastUpdated *timestamppb.Timestamp
		if entry != nil {
			lastUpdated = timestamppb.New(entry.CreatedAt)
		}

		return &api.ListRoadsResponse{
			Roads:       cachedRoads,
			LastUpdated: lastUpdated,
		}, nil
	}

	// Cache miss or stale - refresh from external APIs
	log.Printf("Refreshing road data from external APIs")
	roads, err := s.refreshRoadData(ctx)
	if err != nil {
		// If refresh fails but we have stale cached data, return it
		if found && !s.cache.IsVeryStale(cacheKey) {
			log.Printf("Refresh failed, returning stale cached roads: %v", err)
			entry, _, _ := s.cache.GetWithMetadata(cacheKey, nil)
			var lastUpdated *timestamppb.Timestamp
			if entry != nil {
				lastUpdated = timestamppb.New(entry.CreatedAt)
			}

			return &api.ListRoadsResponse{
				Roads:       cachedRoads,
				LastUpdated: lastUpdated,
			}, nil
		}
		return nil, fmt.Errorf("failed to refresh road data: %w", err)
	}

	// Cache the refreshed data
	if err := s.cache.Set(cacheKey, roads, s.config.GoogleRoutes.RefreshInterval, "roads"); err != nil {
		log.Printf("Failed to cache roads: %v", err)
	}

	return &api.ListRoadsResponse{
		Roads:       roads,
		LastUpdated: timestamppb.Now(),
	}, nil
}

// GetRoad implements the gRPC method for retrieving a specific road
func (s *RoadsService) GetRoad(ctx context.Context, req *api.GetRoadRequest) (*api.GetRoadResponse, error) {
	log.Printf("GetRoad called for road ID: %s", req.RoadId)

	// Get all roads (will use cache if available)
	listResp, err := s.ListRoads(ctx, &api.ListRoadsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get roads: %w", err)
	}

	// Find the requested road
	for _, road := range listResp.Roads {
		if road.Id == req.RoadId {
			return &api.GetRoadResponse{
				Road:        road,
				LastUpdated: listResp.LastUpdated,
			}, nil
		}
	}

	return nil, fmt.Errorf("road not found: %s", req.RoadId)
}

// GetProcessingMetrics implements the gRPC method for processing metrics
func (s *RoadsService) GetProcessingMetrics(ctx context.Context, req *api.GetProcessingMetricsRequest) (*api.ProcessingMetrics, error) {
	log.Printf("GetProcessingMetrics called")

	// TODO: Implement proper metrics collection
	// For now, return placeholder metrics
	return &api.ProcessingMetrics{
		TotalRawAlerts:      0,
		FilteredAlerts:      0,
		EnhancedAlerts:      0,
		EnhancementFailures: 0,
		AvgProcessingTimeMs: 0.0,
	}, nil
}

// refreshRoadData fetches fresh data from all external sources
func (s *RoadsService) refreshRoadData(ctx context.Context) ([]*api.Road, error) {
	var roads []*api.Road

	// Process each monitored road
	for _, monitoredRoad := range s.config.MonitoredRoads {
		road, err := s.processMonitoredRoad(ctx, monitoredRoad)
		if err != nil {
			log.Printf("Failed to process road %s: %v", monitoredRoad.ID, err)
			// Continue processing other roads even if one fails
			continue
		}
		roads = append(roads, road)
	}

	if len(roads) == 0 {
		return nil, fmt.Errorf("no roads could be processed")
	}

	return roads, nil
}

// processMonitoredRoad processes a single road with all data sources
func (s *RoadsService) processMonitoredRoad(ctx context.Context, monitoredRoad config.MonitoredRoad) (*api.Road, error) {
	log.Printf("Processing road: %s", monitoredRoad.ID)

	// Get traffic data and route geometry from Google Routes
	durationMins, distanceKm, congestionLevel, delayMins, googlePolyline, err := s.getTrafficDataWithPolyline(ctx, monitoredRoad)
	if err != nil {
		log.Printf("Failed to get traffic data for %s: %v", monitoredRoad.ID, err)
		// Use defaults for missing traffic data
		durationMins = 0
		distanceKm = 0
		congestionLevel = "unknown"
		delayMins = 0
		googlePolyline = "" // Will fall back to simple 2-point polyline
	}

	// Get Caltrans data for road status and chain control using actual route geometry
	roadStatus, chainControl, alerts, err := s.getCaltransDataWithRouteGeometry(ctx, monitoredRoad, googlePolyline)
	if err != nil {
		log.Printf("Failed to get Caltrans data for %s: %v", monitoredRoad.ID, err)
		// Use defaults
		roadStatus = "open"
		chainControl = "none"
		alerts = nil
	}

	// Build road object (internal fields like origin, destination, polylines kept internal)
	road := &api.Road{
		Id:              monitoredRoad.ID,
		Name:            monitoredRoad.Name,
		Section:         monitoredRoad.Section,
		Status:          s.mapRoadStatus(roadStatus),
		DurationMinutes: durationMins,
		DistanceKm:      distanceKm,
		CongestionLevel: s.mapCongestionLevel(congestionLevel),
		DelayMinutes:    delayMins,
		ChainControl:    s.mapChainControlStatus(chainControl),
		Alerts:          alerts,
	}

	return road, nil
}

// getTrafficDataWithPolyline fetches traffic data and route geometry from Google Routes API
func (s *RoadsService) getTrafficDataWithPolyline(ctx context.Context, monitoredRoad config.MonitoredRoad) (int32, int32, string, int32, string, error) {
	if s.config.GoogleRoutes.APIKey == "" {
		return 0, 0, "unknown", 0, "", fmt.Errorf("google Routes API key not configured")
	}

	roadData, err := s.googleClient.ComputeRoutes(ctx,
		monitoredRoad.Origin.ToProto(),
		monitoredRoad.Destination.ToProto())
	if err != nil {
		return 0, 0, "unknown", 0, "", fmt.Errorf("failed to compute routes: %w", err)
	}

	// Calculate real delay from Google's traffic-aware vs baseline durations
	delaySeconds := roadData.DurationSeconds - roadData.StaticDurationSeconds
	if delaySeconds < 0 {
		delaySeconds = 0 // Shouldn't happen, but safety check
	}

	// Determine congestion level based on actual delay minutes
	delayMins := int32(delaySeconds / 60)
	congestionLevel := s.classifyCongestionByDelay(delayMins)

	// Convert to user-friendly units
	durationMins := int32(roadData.DurationSeconds / 60)
	distanceKm := int32(roadData.DistanceMeters / 1000)

	return durationMins, distanceKm, congestionLevel, delayMins, roadData.Polyline, nil
}

// classifyCongestionByDelay determines congestion level based on actual delay minutes
func (s *RoadsService) classifyCongestionByDelay(delayMins int32) string {
	switch {
	case delayMins >= 20:
		return "severe"  // 20+ minutes delay
	case delayMins >= 10:
		return "heavy"   // 10-19 minutes delay
	case delayMins >= 5:
		return "moderate" // 5-9 minutes delay
	case delayMins >= 2:
		return "light"   // 2-4 minutes delay
	default:
		return "clear"   // 0-1 minutes delay
	}
}


// mapRoadStatus converts string status to RoadStatus enum
func (s *RoadsService) mapRoadStatus(status string) api.RoadStatus {
	switch status {
	case "open":
		return api.RoadStatus_OPEN
	case "closed":
		return api.RoadStatus_CLOSED
	case "restricted":
		return api.RoadStatus_RESTRICTED
	case "maintenance":
		return api.RoadStatus_MAINTENANCE
	default:
		return api.RoadStatus_ROAD_STATUS_UNSPECIFIED
	}
}

// mapCongestionLevel converts string congestion level to CongestionLevel enum
func (s *RoadsService) mapCongestionLevel(level string) api.CongestionLevel {
	switch level {
	case "clear":
		return api.CongestionLevel_CLEAR
	case "light":
		return api.CongestionLevel_LIGHT
	case "moderate":
		return api.CongestionLevel_MODERATE
	case "heavy":
		return api.CongestionLevel_HEAVY
	case "severe":
		return api.CongestionLevel_SEVERE
	default:
		return api.CongestionLevel_CONGESTION_LEVEL_UNSPECIFIED
	}
}

// mapChainControlStatus converts string chain control to ChainControlStatus enum
func (s *RoadsService) mapChainControlStatus(status string) api.ChainControlStatus {
	switch status {
	case "none":
		return api.ChainControlStatus_NONE
	case "advised":
		return api.ChainControlStatus_ADVISED
	case "required":
		return api.ChainControlStatus_REQUIRED
	case "prohibited":
		return api.ChainControlStatus_PROHIBITED
	default:
		return api.ChainControlStatus_CHAIN_CONTROL_UNSPECIFIED
	}
}

// Removed duplicate analyzeCongestionLevel - now combined above

// getCaltransDataWithRouteGeometry fetches road status, chain control, and alerts using actual route geometry
func (s *RoadsService) getCaltransDataWithRouteGeometry(ctx context.Context, monitoredRoad config.MonitoredRoad, googlePolyline string) (string, string, []*api.RoadAlert, error) {
	// Create route definition for classification using actual Google polyline if available
	var routePolyline geo.Polyline
	if googlePolyline != "" {
		// Decode Google polyline to get actual route points
		decodedPoints, err := s.geoUtils.DecodePolyline(googlePolyline)
		if err != nil {
			log.Printf("Failed to decode Google polyline for %s: %v", monitoredRoad.ID, err)
			// Fall back to simple 2-point polyline
			routePolyline = geo.Polyline{Points: []geo.Point{
				{Latitude: monitoredRoad.Origin.Latitude, Longitude: monitoredRoad.Origin.Longitude},
				{Latitude: monitoredRoad.Destination.Latitude, Longitude: monitoredRoad.Destination.Longitude},
			}}
		} else {
			routePolyline = geo.Polyline{Points: decodedPoints}
		}
	} else {
		// Use simple 2-point polyline as fallback
		routePolyline = geo.Polyline{Points: []geo.Point{
			{Latitude: monitoredRoad.Origin.Latitude, Longitude: monitoredRoad.Origin.Longitude},
			{Latitude: monitoredRoad.Destination.Latitude, Longitude: monitoredRoad.Destination.Longitude},
		}}
	}

	route := routing.Route{
		ID:          monitoredRoad.ID,
		Name:        monitoredRoad.Name,
		Section:     monitoredRoad.Section,
		Origin:      geo.Point{Latitude: monitoredRoad.Origin.Latitude, Longitude: monitoredRoad.Origin.Longitude},
		Destination: geo.Point{Latitude: monitoredRoad.Destination.Latitude, Longitude: monitoredRoad.Destination.Longitude},
		Polyline:    routePolyline,
		MaxDistance: 10000, // Default 10 kilometers
	}

	return s.processCaltransDataWithRoute(ctx, route, monitoredRoad)
}

// getCaltransData fetches road status, chain control, and alerts using route-aware classification (legacy method)
func (s *RoadsService) getCaltransData(ctx context.Context, monitoredRoad config.MonitoredRoad) (string, string, []*api.RoadAlert, error) {
	return s.getCaltransDataWithRouteGeometry(ctx, monitoredRoad, "")
}

// processCaltransDataWithRoute handles the actual Caltrans data processing with route information
func (s *RoadsService) processCaltransDataWithRoute(ctx context.Context, route routing.Route, monitoredRoad config.MonitoredRoad) (string, string, []*api.RoadAlert, error) {

	// Get all incidents from Caltrans (no geographic pre-filtering)
	laneClosures, _ := s.caltransClient.ParseLaneClosures(ctx)
	chpIncidents, _ := s.caltransClient.ParseCHPIncidents(ctx)

	// Combine all incidents
	allIncidents := append(laneClosures, chpIncidents...)

	// Convert Caltrans incidents to unclassified alerts
	var unclassifiedAlerts []routing.UnclassifiedAlert
	for _, incident := range allIncidents {
		unclassifiedAlert := routing.UnclassifiedAlert{
			ID:          fmt.Sprintf("%s_%d", incident.Name, incident.LastFetched.Unix()),
			Title:       incident.Name, // Use actual Caltrans title (e.g., "CHP Incident 250911GG0206")
			Location:    geo.Point{Latitude: incident.Coordinates.Latitude, Longitude: incident.Coordinates.Longitude},
			Description: incident.DescriptionText,
			Type:        s.mapCaltransTypeToString(incident.FeedType),
			StyleUrl:    incident.StyleUrl,
		}

		// Add affected polyline if available
		if incident.AffectedArea != nil {
			geoPolyline := geo.Polyline{Points: make([]geo.Point, len(incident.AffectedArea.Points))}
			for i, point := range incident.AffectedArea.Points {
				geoPolyline.Points[i] = geo.Point{Latitude: point.Latitude, Longitude: point.Longitude}
			}
			unclassifiedAlert.AffectedPolyline = &geoPolyline
		}

		unclassifiedAlerts = append(unclassifiedAlerts, unclassifiedAlert)
	}

	// Classify alerts using route-aware matching
	var classifiedAlerts []routing.ClassifiedAlert
	for _, unclassifiedAlert := range unclassifiedAlerts {
		classifiedAlert, err := s.routeMatcher.ClassifyAlert(ctx, unclassifiedAlert, []routing.Route{route})
		if err != nil {
			log.Printf("Error classifying alert %s: %v", unclassifiedAlert.ID, err)
			continue
		}
		classifiedAlerts = append(classifiedAlerts, classifiedAlert)
	}

	// Process classified alerts
	roadStatus := "open"
	chainControl := "none"
	var enhancedAlerts []*api.RoadAlert

	for _, classifiedAlert := range classifiedAlerts {
		// Only include ON_ROUTE and NEARBY alerts
		if classifiedAlert.Classification == routing.Distant {
			continue
		}

		// Convert to API road alert
		alert, err := s.buildEnhancedRoadAlert(ctx, classifiedAlert, monitoredRoad)
		if err != nil {
			log.Printf("Error building enhanced alert: %v", err)
			continue
		}

		enhancedAlerts = append(enhancedAlerts, alert)

		// Update road status based on classified alerts
		if classifiedAlert.Classification == routing.OnRoute &&
			(classifiedAlert.Type == "closure" || classifiedAlert.Type == "construction") {
			roadStatus = "restricted"
		}
	}

	return roadStatus, chainControl, enhancedAlerts, nil
}

// mapCaltransTypeToString converts Caltrans feed type to string
func (s *RoadsService) mapCaltransTypeToString(feedType caltrans.CaltransFeedType) string {
	switch feedType {
	case caltrans.CHAIN_CONTROL:
		return "weather"
	case caltrans.LANE_CLOSURE:
		return "closure"
	case caltrans.CHP_INCIDENT:
		return "incident"
	default:
		return "unknown"
	}
}

// buildEnhancedRoadAlert creates an enhanced API road alert from classified alert
func (s *RoadsService) buildEnhancedRoadAlert(ctx context.Context, classifiedAlert routing.ClassifiedAlert, monitoredRoad config.MonitoredRoad) (*api.RoadAlert, error) {
	startTime := time.Now() // Default to current time

	// Build base alert (polylines kept internal for processing)
	alertType := s.mapStringToAlertType(classifiedAlert.Type)
	alert := &api.RoadAlert{
		Type:           alertType,
		Severity:       api.AlertSeverity_WARNING, // Default, will be updated after AI enhancement
		Classification: s.mapRoutingToAPIClassification(classifiedAlert.Classification),
		Title:          classifiedAlert.Title, // Use real Caltrans title (e.g., "CHP Incident 250911GG0206")
		Description:    classifiedAlert.Description, // Will be enhanced below
		StartTime:      timestamppb.New(startTime),
		EndTime:        nil,
		LastUpdated:    timestamppb.Now(),
		Location:       &api.Coordinates{Latitude: classifiedAlert.Location.Latitude, Longitude: classifiedAlert.Location.Longitude},
		Metadata:       make(map[string]string),
		// Note: ID, OriginalDescription removed for cleaner API
		// Note: AffectedSegments, DistanceToRoute, AffectedRouteIds, AffectedPolyline kept internal
	}

	// Enhance with AI if available
	if s.alertEnhancer != nil {
		enhanced, err := s.enhanceAlertWithAI(ctx, classifiedAlert)
		if err != nil {
			log.Printf("Alert enhancement failed, using original: %v", err)
		} else {
			// Update alert with enhanced data at top level
			alert.Description = enhanced.StructuredDescription.Details
			alert.CondensedSummary = enhanced.CondensedSummary
			alert.LocationDescription = enhanced.StructuredDescription.Location.Description
			alert.Impact = enhanced.StructuredDescription.Impact
			alert.Duration = enhanced.StructuredDescription.Duration

			// Parse time_reported if provided
			if enhanced.StructuredDescription.TimeReported != "" {
				if timeReported, err := time.Parse(time.RFC3339, enhanced.StructuredDescription.TimeReported); err == nil {
					alert.TimeReported = timestamppb.New(timeReported)
				}
			}

			// Update severity based on AI-enhanced impact and description
			alert.Severity = s.determineAlertSeverity(
				classifiedAlert.Classification,
				enhanced.StructuredDescription.Impact,
				alertType,
				enhanced.StructuredDescription.Details,
			)

			// Reserve metadata only for AI's additional_info
			for key, value := range enhanced.StructuredDescription.AdditionalInfo {
				alert.Metadata[key] = value
			}
		}
	}

	return alert, nil
}

// enhanceAlertWithAI uses the alert enhancer to improve alert descriptions
func (s *RoadsService) enhanceAlertWithAI(ctx context.Context, classifiedAlert routing.ClassifiedAlert) (*alerts.EnhancedAlert, error) {
	rawAlert := alerts.RawAlert{
		ID:          classifiedAlert.ID,
		Description: classifiedAlert.Description,
		Location:    fmt.Sprintf("Highway alert near %.4f, %.4f", classifiedAlert.Location.Latitude, classifiedAlert.Location.Longitude),
		StyleUrl:    classifiedAlert.StyleUrl,
		Timestamp:   time.Now(),
	}

	enhanced, err := s.alertEnhancer.EnhanceAlert(ctx, rawAlert)
	if err != nil {
		return nil, err
	}
	return &enhanced, nil
}

// Helper mapping functions
func (s *RoadsService) mapStringToAlertType(typeStr string) api.AlertType {
	switch typeStr {
	case "closure":
		return api.AlertType_CLOSURE
	case "construction":
		return api.AlertType_CONSTRUCTION
	case "incident":
		return api.AlertType_INCIDENT
	case "weather":
		return api.AlertType_WEATHER
	default:
		return api.AlertType_ALERT_TYPE_UNSPECIFIED
	}
}

func (s *RoadsService) determineAlertSeverity(classification routing.AlertClassification, impact string, alertType api.AlertType, description string) api.AlertSeverity {
	// Base severity on impact level first, then adjust by classification
	var baseSeverity api.AlertSeverity
	
	switch impact {
	case "severe":
		baseSeverity = api.AlertSeverity_CRITICAL
	case "moderate":
		baseSeverity = api.AlertSeverity_WARNING
	case "light":
		baseSeverity = api.AlertSeverity_WARNING
	case "none":
		baseSeverity = api.AlertSeverity_INFO
	default:
		// Fallback based on alert type and description
		if alertType == api.AlertType_CLOSURE {
			baseSeverity = api.AlertSeverity_CRITICAL
		} else if strings.Contains(strings.ToLower(description), "assistance") ||
			strings.Contains(strings.ToLower(description), "maintenance") {
			baseSeverity = api.AlertSeverity_INFO
		} else {
			baseSeverity = api.AlertSeverity_WARNING
		}
	}
	
	// Downgrade if distant (far from route)
	if classification == routing.Distant {
		baseSeverity = api.AlertSeverity_INFO
	}
	
	return baseSeverity
}

func (s *RoadsService) mapRoutingToAPIClassification(classification routing.AlertClassification) api.AlertClassification {
	switch classification {
	case routing.OnRoute:
		return api.AlertClassification_ON_ROUTE
	case routing.Nearby:
		return api.AlertClassification_NEARBY
	case routing.Distant:
		return api.AlertClassification_DISTANT
	default:
		return api.AlertClassification_ALERT_CLASSIFICATION_UNSPECIFIED
	}
}

// Legacy mapping functions - kept for compatibility but replaced by route-aware classification

// TODO: Re-enable for winter chain control processing
// extractChainControlStatus determines chain control requirements from description
// func (s *RoadsService) extractChainControlStatus(description string) string {
// 	lowerDesc := strings.ToLower(description)
//
// 	if strings.Contains(lowerDesc, "chain control required") ||
// 	   strings.Contains(lowerDesc, "chains required") {
// 		return "required"
// 	}
// 	if strings.Contains(lowerDesc, "chain control advised") ||
// 	   strings.Contains(lowerDesc, "chains advised") {
// 		return "advised"
// 	}
// 	if strings.Contains(lowerDesc, "chain control in effect") {
// 		return "required"
// 	}
//
// 	return "none"
// }
