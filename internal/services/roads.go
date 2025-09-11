package services

import (
	"context"
	"fmt"
	"log"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/cache"
	"github.com/dpup/info.ersn.net/server/internal/clients/caltrans"
	"github.com/dpup/info.ersn.net/server/internal/clients/google"
	"github.com/dpup/info.ersn.net/server/internal/config"
)

// RoadsService implements the gRPC RoadsService
// Implementation per tasks.md T016 and data-model.md Route entity
type RoadsService struct {
	api.UnimplementedRoadsServiceServer
	googleClient   *google.Client
	caltransClient *caltrans.FeedParser
	cache          *cache.Cache
	config         *config.RoadsConfig
}

// NewRoadsService creates a new RoadsService
func NewRoadsService(googleClient *google.Client, caltransClient *caltrans.FeedParser, cache *cache.Cache, config *config.RoadsConfig) *RoadsService {
	return &RoadsService{
		googleClient:   googleClient,
		caltransClient: caltransClient,
		cache:          cache,
		config:         config,
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

	// Get traffic data from Google Routes
	durationMins, distanceKm, congestionLevel, delayMins, err := s.getTrafficData(ctx, monitoredRoad)
	if err != nil {
		log.Printf("Failed to get traffic data for %s: %v", monitoredRoad.ID, err)
		// Use defaults for missing traffic data
		durationMins = 0
		distanceKm = 0
		congestionLevel = "unknown"
		delayMins = 0
	}

	// Get Caltrans data for road status and chain control
	roadStatus, chainControl, alerts, err := s.getCaltransData(ctx, monitoredRoad)
	if err != nil {
		log.Printf("Failed to get Caltrans data for %s: %v", monitoredRoad.ID, err)
		// Use defaults
		roadStatus = "open"
		chainControl = "none"
		alerts = nil
	}

	// Build road object with enum-based structure
	road := &api.Road{
		Id:              monitoredRoad.ID,
		Name:            monitoredRoad.Name,
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

// getTrafficData fetches traffic data from Google Routes API
func (s *RoadsService) getTrafficData(ctx context.Context, monitoredRoad config.MonitoredRoad) (int32, int32, string, int32, error) {
	if s.config.GoogleRoutes.APIKey == "" {
		return 0, 0, "unknown", 0, fmt.Errorf("Google Routes API key not configured")
	}

	roadData, err := s.googleClient.ComputeRoutes(ctx, 
		monitoredRoad.Origin.ToProto(),
		monitoredRoad.Destination.ToProto())
	if err != nil {
		return 0, 0, "unknown", 0, fmt.Errorf("failed to compute routes: %w", err)
	}

	// Determine congestion level based on speed readings
	congestionLevel := s.analyzeCongestionLevel(roadData.SpeedReadings)

	// Calculate delay (simplified - could be enhanced with historical data)
	delaySeconds := int32(0)
	switch congestionLevel {
	case "moderate":
		delaySeconds = int32(float32(roadData.DurationSeconds) * 0.10)
	case "heavy":
		delaySeconds = int32(float32(roadData.DurationSeconds) * 0.25)
	case "severe":
		delaySeconds = int32(float32(roadData.DurationSeconds) * 0.50)
	}

	// Convert to user-friendly units
	durationMins := int32(roadData.DurationSeconds / 60)
	distanceKm := int32(roadData.DistanceMeters / 1000)
	delayMins := int32(delaySeconds / 60)
	
	return durationMins, distanceKm, congestionLevel, delayMins, nil
}

// analyzeCongestionLevel determines traffic congestion from speed readings and returns simple string
func (s *RoadsService) analyzeCongestionLevel(speedReadings []google.SpeedReading) string {
	if len(speedReadings) == 0 {
		return "clear"
	}

	// Count different speed categories
	var normal, slow, trafficJam int
	for _, reading := range speedReadings {
		switch reading.SpeedCategory {
		case "NORMAL":
			normal++
		case "SLOW":
			slow++
		case "TRAFFIC_JAM":
			trafficJam++
		}
	}

	total := len(speedReadings)
	
	// Determine overall congestion based on proportions
	if trafficJam*100/total > 30 { // More than 30% traffic jams
		return "severe"
	} else if trafficJam*100/total > 10 || slow*100/total > 50 { // 10%+ jams or 50%+ slow
		return "heavy"
	} else if slow*100/total > 20 { // 20%+ slow traffic
		return "moderate"
	} else if slow*100/total > 5 { // Some slow traffic
		return "light"
	}

	return "clear"
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

// getCaltransData fetches road status, chain control, and alerts from Caltrans
func (s *RoadsService) getCaltransData(ctx context.Context, monitoredRoad config.MonitoredRoad) (string, string, []*api.RoadAlert, error) {
	// Define road coordinates for geographic filtering
	roadCoordinates := []struct{ Lat, Lon float64 }{
		{monitoredRoad.Origin.Latitude, monitoredRoad.Origin.Longitude},
		{monitoredRoad.Destination.Latitude, monitoredRoad.Destination.Longitude},
	}

	// Get incidents within 50km of road
	incidents, err := s.caltransClient.ParseWithGeographicFilter(ctx, roadCoordinates, 50000)
	if err != nil {
		return "open", "none", nil, err
	}

	// Process incidents to determine road status and chain control
	roadStatus := "open"
	chainControl := "none"
	var alerts []*api.RoadAlert

	for _, incident := range incidents {
		// Convert to road alert
		alert := &api.RoadAlert{
			Id:          fmt.Sprintf("%s_%d", incident.Name, incident.LastFetched.Unix()),
			Type:        s.mapIncidentToAlertType(incident),
			Severity:    s.mapIncidentToSeverity(incident),
			Title:       incident.Name,
			Description: incident.DescriptionText,
			StartTime:   timestamppb.New(incident.LastFetched), // Use fetch time as proxy
			EndTime:     nil, // Usually not available in KML
			AffectedSegments: []string{monitoredRoad.Name}, // Simplified
		}
		alerts = append(alerts, alert)

		// Update route status based on incident type
		if incident.FeedType == caltrans.CHAIN_CONTROL {
			chainControl = s.extractChainControlStatus(incident.DescriptionText)
		}

		// Update road status for closures
		if incident.FeedType == caltrans.LANE_CLOSURE && 
			 (incident.ParsedStatus == "closed" || incident.ParsedStatus == "closure") {
			roadStatus = "restricted"
		}
	}

	return roadStatus, chainControl, alerts, nil
}

// mapIncidentToAlertType maps Caltrans incident types to our alert types
func (s *RoadsService) mapIncidentToAlertType(incident caltrans.CaltransIncident) api.AlertType {
	switch incident.FeedType {
	case caltrans.CHAIN_CONTROL:
		return api.AlertType_WEATHER
	case caltrans.LANE_CLOSURE:
		if incident.ParsedStatus == "construction" {
			return api.AlertType_CONSTRUCTION
		}
		return api.AlertType_CLOSURE
	case caltrans.CHP_INCIDENT:
		return api.AlertType_INCIDENT
	default:
		return api.AlertType_ALERT_TYPE_UNSPECIFIED
	}
}

// mapIncidentToSeverity determines alert severity from incident data
func (s *RoadsService) mapIncidentToSeverity(incident caltrans.CaltransIncident) api.AlertSeverity {
	if incident.ParsedStatus == "closed" || incident.ParsedStatus == "closure" {
		return api.AlertSeverity_CRITICAL
	}
	if incident.FeedType == caltrans.CHP_INCIDENT {
		return api.AlertSeverity_WARNING
	}
	return api.AlertSeverity_INFO
}

// extractChainControlStatus determines chain control requirements from description
func (s *RoadsService) extractChainControlStatus(description string) string {
	lowerDesc := strings.ToLower(description)
	
	if strings.Contains(lowerDesc, "chain control required") || 
	   strings.Contains(lowerDesc, "chains required") {
		return "required"
	}
	if strings.Contains(lowerDesc, "chain control advised") || 
	   strings.Contains(lowerDesc, "chains advised") {
		return "advised"
	}
	if strings.Contains(lowerDesc, "chain control in effect") {
		return "required"
	}
	
	return "none"
}