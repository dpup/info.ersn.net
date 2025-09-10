package services

import (
	"context"
	"fmt"
	"log"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/dpup/info.ersn.net/server"
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
	trafficCondition, err := s.getTrafficCondition(ctx, monitoredRoad)
	if err != nil {
		log.Printf("Failed to get traffic data for %s: %v", monitoredRoad.ID, err)
		// Continue without traffic data rather than failing completely
	}

	// Get Caltrans data for road status and chain control
	roadStatus, chainControl, alerts, err := s.getCaltransData(ctx, monitoredRoad)
	if err != nil {
		log.Printf("Failed to get Caltrans data for %s: %v", monitoredRoad.ID, err)
		// Use defaults
		roadStatus = api.RoadStatus_ROAD_STATUS_OPEN
		chainControl = api.ChainControlStatus_CHAIN_CONTROL_NONE
		alerts = nil
	}

	// Build road object
	road := &api.Road{
		Id:               monitoredRoad.ID,
		Name:             monitoredRoad.Name,
		Origin:           monitoredRoad.Origin.ToProto(),
		Destination:      monitoredRoad.Destination.ToProto(),
		Status:           roadStatus,
		TrafficCondition: trafficCondition,
		ChainControl:     chainControl,
		Alerts:           alerts,
		LastUpdated:      timestamppb.Now(),
	}

	return road, nil
}

// getTrafficCondition fetches traffic data from Google Routes API
func (s *RoadsService) getTrafficCondition(ctx context.Context, monitoredRoad config.MonitoredRoad) (*api.TrafficCondition, error) {
	if s.config.GoogleRoutes.APIKey == "" {
		return nil, fmt.Errorf("Google Routes API key not configured")
	}

	roadData, err := s.googleClient.ComputeRoutes(ctx, 
		monitoredRoad.Origin.ToProto(),
		monitoredRoad.Destination.ToProto())
	if err != nil {
		return nil, fmt.Errorf("failed to compute routes: %w", err)
	}

	// Determine congestion level based on speed readings
	congestionLevel := s.analyzeCongestionLevel(roadData.SpeedReadings)

	// Calculate delay (simplified - could be enhanced with historical data)
	delaySeconds := int32(0)
	if congestionLevel >= api.CongestionLevel_CONGESTION_LEVEL_MODERATE {
		// Rough estimation: 10% delay for moderate, 25% for heavy, 50% for severe
		delayMultiplier := map[api.CongestionLevel]float32{
			api.CongestionLevel_CONGESTION_LEVEL_MODERATE: 0.10,
			api.CongestionLevel_CONGESTION_LEVEL_HEAVY:    0.25,
			api.CongestionLevel_CONGESTION_LEVEL_SEVERE:   0.50,
		}
		if multiplier, ok := delayMultiplier[congestionLevel]; ok {
			delaySeconds = int32(float32(roadData.DurationSeconds) * multiplier)
		}
	}

	return &api.TrafficCondition{
		RoadId:          monitoredRoad.ID,
		DurationSeconds: roadData.DurationSeconds,
		DistanceMeters:  roadData.DistanceMeters,
		CongestionLevel: congestionLevel,
		DelaySeconds:    delaySeconds,
	}, nil
}

// analyzeCongestionLevel determines traffic congestion from speed readings
func (s *RoadsService) analyzeCongestionLevel(speedReadings []google.SpeedReading) api.CongestionLevel {
	if len(speedReadings) == 0 {
		return api.CongestionLevel_CONGESTION_LEVEL_CLEAR
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
		return api.CongestionLevel_CONGESTION_LEVEL_SEVERE
	} else if trafficJam*100/total > 10 || slow*100/total > 50 { // 10%+ jams or 50%+ slow
		return api.CongestionLevel_CONGESTION_LEVEL_HEAVY
	} else if slow*100/total > 20 { // 20%+ slow traffic
		return api.CongestionLevel_CONGESTION_LEVEL_MODERATE
	} else if slow*100/total > 5 { // Some slow traffic
		return api.CongestionLevel_CONGESTION_LEVEL_LIGHT
	}

	return api.CongestionLevel_CONGESTION_LEVEL_CLEAR
}

// getCaltransData fetches road status, chain control, and alerts from Caltrans
func (s *RoadsService) getCaltransData(ctx context.Context, monitoredRoad config.MonitoredRoad) (api.RoadStatus, api.ChainControlStatus, []*api.RoadAlert, error) {
	// Define road coordinates for geographic filtering
	roadCoordinates := []struct{ Lat, Lon float64 }{
		{monitoredRoad.Origin.Latitude, monitoredRoad.Origin.Longitude},
		{monitoredRoad.Destination.Latitude, monitoredRoad.Destination.Longitude},
	}

	// Get incidents within 50km of road
	incidents, err := s.caltransClient.ParseWithGeographicFilter(ctx, roadCoordinates, 50000)
	if err != nil {
		return api.RoadStatus_ROAD_STATUS_OPEN, api.ChainControlStatus_CHAIN_CONTROL_NONE, nil, err
	}

	// Process incidents to determine road status and chain control
	roadStatus := api.RoadStatus_ROAD_STATUS_OPEN
	chainControl := api.ChainControlStatus_CHAIN_CONTROL_NONE
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
			roadStatus = api.RoadStatus_ROAD_STATUS_RESTRICTED
		}
	}

	return roadStatus, chainControl, alerts, nil
}

// mapIncidentToAlertType maps Caltrans incident types to our alert types
func (s *RoadsService) mapIncidentToAlertType(incident caltrans.CaltransIncident) api.AlertType {
	switch incident.FeedType {
	case caltrans.CHAIN_CONTROL:
		return api.AlertType_ALERT_TYPE_WEATHER
	case caltrans.LANE_CLOSURE:
		if incident.ParsedStatus == "construction" {
			return api.AlertType_ALERT_TYPE_CONSTRUCTION
		}
		return api.AlertType_ALERT_TYPE_CLOSURE
	case caltrans.CHP_INCIDENT:
		return api.AlertType_ALERT_TYPE_INCIDENT
	default:
		return api.AlertType_ALERT_TYPE_INCIDENT
	}
}

// mapIncidentToSeverity determines alert severity from incident data
func (s *RoadsService) mapIncidentToSeverity(incident caltrans.CaltransIncident) api.AlertSeverity {
	if incident.ParsedStatus == "closed" || incident.ParsedStatus == "closure" {
		return api.AlertSeverity_ALERT_SEVERITY_CRITICAL
	}
	if incident.FeedType == caltrans.CHP_INCIDENT {
		return api.AlertSeverity_ALERT_SEVERITY_WARNING
	}
	return api.AlertSeverity_ALERT_SEVERITY_INFO
}

// extractChainControlStatus determines chain control requirements from description
func (s *RoadsService) extractChainControlStatus(description string) api.ChainControlStatus {
	lowerDesc := strings.ToLower(description)
	
	if strings.Contains(lowerDesc, "chain control required") || 
	   strings.Contains(lowerDesc, "chains required") {
		return api.ChainControlStatus_CHAIN_CONTROL_REQUIRED
	}
	if strings.Contains(lowerDesc, "chain control advised") || 
	   strings.Contains(lowerDesc, "chains advised") {
		return api.ChainControlStatus_CHAIN_CONTROL_ADVISED
	}
	if strings.Contains(lowerDesc, "chain control in effect") {
		return api.ChainControlStatus_CHAIN_CONTROL_REQUIRED
	}
	
	return api.ChainControlStatus_CHAIN_CONTROL_NONE
}