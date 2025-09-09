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
	config         *config.RoutesConfig
}

// NewRoadsService creates a new RoadsService
func NewRoadsService(googleClient *google.Client, caltransClient *caltrans.FeedParser, cache *cache.Cache, config *config.RoutesConfig) *RoadsService {
	return &RoadsService{
		googleClient:   googleClient,
		caltransClient: caltransClient,
		cache:          cache,
		config:         config,
	}
}

// ListRoutes implements the gRPC method defined in contracts/roads.proto line 12-17
func (s *RoadsService) ListRoutes(ctx context.Context, req *api.ListRoutesRequest) (*api.ListRoutesResponse, error) {
	log.Printf("ListRoutes called")

	// Try to get cached routes first
	var cachedRoutes []*api.Route
	cacheKey := "routes:all"
	
	found, err := s.cache.Get(cacheKey, &cachedRoutes)
	if err != nil {
		log.Printf("Cache error: %v", err)
	}

	if found && !s.cache.IsStale(cacheKey) {
		log.Printf("Returning cached routes (%d routes)", len(cachedRoutes))
		
		// Get cache metadata for last_updated timestamp
		entry, _, _ := s.cache.GetWithMetadata(cacheKey, nil)
		var lastUpdated *timestamppb.Timestamp
		if entry != nil {
			lastUpdated = timestamppb.New(entry.CreatedAt)
		}

		return &api.ListRoutesResponse{
			Routes:      cachedRoutes,
			LastUpdated: lastUpdated,
		}, nil
	}

	// Cache miss or stale - refresh from external APIs
	log.Printf("Refreshing route data from external APIs")
	routes, err := s.refreshRouteData(ctx)
	if err != nil {
		// If refresh fails but we have stale cached data, return it
		if found && !s.cache.IsVeryStale(cacheKey) {
			log.Printf("Refresh failed, returning stale cached routes: %v", err)
			entry, _, _ := s.cache.GetWithMetadata(cacheKey, nil)
			var lastUpdated *timestamppb.Timestamp
			if entry != nil {
				lastUpdated = timestamppb.New(entry.CreatedAt)
			}

			return &api.ListRoutesResponse{
				Routes:      cachedRoutes,
				LastUpdated: lastUpdated,
			}, nil
		}
		return nil, fmt.Errorf("failed to refresh route data: %w", err)
	}

	// Cache the refreshed data
	if err := s.cache.Set(cacheKey, routes, s.config.GoogleRoutes.RefreshInterval, "routes"); err != nil {
		log.Printf("Failed to cache routes: %v", err)
	}

	return &api.ListRoutesResponse{
		Routes:      routes,
		LastUpdated: timestamppb.Now(),
	}, nil
}

// GetRoute implements the gRPC method for retrieving a specific route
func (s *RoadsService) GetRoute(ctx context.Context, req *api.GetRouteRequest) (*api.GetRouteResponse, error) {
	log.Printf("GetRoute called for route ID: %s", req.RouteId)

	// Get all routes (will use cache if available)
	listResp, err := s.ListRoutes(ctx, &api.ListRoutesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get routes: %w", err)
	}

	// Find the requested route
	for _, route := range listResp.Routes {
		if route.Id == req.RouteId {
			return &api.GetRouteResponse{
				Route:       route,
				LastUpdated: listResp.LastUpdated,
			}, nil
		}
	}

	return nil, fmt.Errorf("route not found: %s", req.RouteId)
}

// refreshRouteData fetches fresh data from all external sources
func (s *RoadsService) refreshRouteData(ctx context.Context) ([]*api.Route, error) {
	var routes []*api.Route

	// Process each monitored route
	for _, monitoredRoute := range s.config.MonitoredRoutes {
		route, err := s.processMonitoredRoute(ctx, monitoredRoute)
		if err != nil {
			log.Printf("Failed to process route %s: %v", monitoredRoute.ID, err)
			// Continue processing other routes even if one fails
			continue
		}
		routes = append(routes, route)
	}

	if len(routes) == 0 {
		return nil, fmt.Errorf("no routes could be processed")
	}

	return routes, nil
}

// processMonitoredRoute processes a single route with all data sources
func (s *RoadsService) processMonitoredRoute(ctx context.Context, monitoredRoute config.MonitoredRoute) (*api.Route, error) {
	log.Printf("Processing route: %s", monitoredRoute.ID)

	// Get traffic data from Google Routes
	trafficCondition, err := s.getTrafficCondition(ctx, monitoredRoute)
	if err != nil {
		log.Printf("Failed to get traffic data for %s: %v", monitoredRoute.ID, err)
		// Continue without traffic data rather than failing completely
	}

	// Get Caltrans data for route status and chain control
	routeStatus, chainControl, alerts, err := s.getCaltransData(ctx, monitoredRoute)
	if err != nil {
		log.Printf("Failed to get Caltrans data for %s: %v", monitoredRoute.ID, err)
		// Use defaults
		routeStatus = api.RouteStatus_ROUTE_STATUS_OPEN
		chainControl = api.ChainControlStatus_CHAIN_CONTROL_NONE
		alerts = nil
	}

	// Build route object
	route := &api.Route{
		Id:               monitoredRoute.ID,
		Name:             monitoredRoute.Name,
		Origin:           monitoredRoute.Origin.ToProto(),
		Destination:      monitoredRoute.Destination.ToProto(),
		Status:           routeStatus,
		TrafficCondition: trafficCondition,
		ChainControl:     chainControl,
		Alerts:           alerts,
		LastUpdated:      timestamppb.Now(),
	}

	return route, nil
}

// getTrafficCondition fetches traffic data from Google Routes API
func (s *RoadsService) getTrafficCondition(ctx context.Context, monitoredRoute config.MonitoredRoute) (*api.TrafficCondition, error) {
	if s.config.GoogleRoutes.APIKey == "" {
		return nil, fmt.Errorf("Google Routes API key not configured")
	}

	routeData, err := s.googleClient.ComputeRoutes(ctx, 
		monitoredRoute.Origin.ToProto(),
		monitoredRoute.Destination.ToProto())
	if err != nil {
		return nil, fmt.Errorf("failed to compute routes: %w", err)
	}

	// Determine congestion level based on speed readings
	congestionLevel := s.analyzeCongestionLevel(routeData.SpeedReadings)

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
			delaySeconds = int32(float32(routeData.DurationSeconds) * multiplier)
		}
	}

	return &api.TrafficCondition{
		RouteId:         monitoredRoute.ID,
		DurationSeconds: routeData.DurationSeconds,
		DistanceMeters:  routeData.DistanceMeters,
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

// getCaltransData fetches route status, chain control, and alerts from Caltrans
func (s *RoadsService) getCaltransData(ctx context.Context, monitoredRoute config.MonitoredRoute) (api.RouteStatus, api.ChainControlStatus, []*api.RouteAlert, error) {
	// Define route coordinates for geographic filtering
	routeCoordinates := []struct{ Lat, Lon float64 }{
		{monitoredRoute.Origin.Latitude, monitoredRoute.Origin.Longitude},
		{monitoredRoute.Destination.Latitude, monitoredRoute.Destination.Longitude},
	}

	// Get incidents within 50km of route
	incidents, err := s.caltransClient.ParseWithGeographicFilter(ctx, routeCoordinates, 50000)
	if err != nil {
		return api.RouteStatus_ROUTE_STATUS_OPEN, api.ChainControlStatus_CHAIN_CONTROL_NONE, nil, err
	}

	// Process incidents to determine route status and chain control
	routeStatus := api.RouteStatus_ROUTE_STATUS_OPEN
	chainControl := api.ChainControlStatus_CHAIN_CONTROL_NONE
	var alerts []*api.RouteAlert

	for _, incident := range incidents {
		// Convert to route alert
		alert := &api.RouteAlert{
			Id:          fmt.Sprintf("%s_%d", incident.Name, incident.LastFetched.Unix()),
			Type:        s.mapIncidentToAlertType(incident),
			Severity:    s.mapIncidentToSeverity(incident),
			Title:       incident.Name,
			Description: incident.DescriptionText,
			StartTime:   timestamppb.New(incident.LastFetched), // Use fetch time as proxy
			EndTime:     nil, // Usually not available in KML
			AffectedSegments: []string{monitoredRoute.Name}, // Simplified
		}
		alerts = append(alerts, alert)

		// Update route status based on incident type
		if incident.FeedType == caltrans.CHAIN_CONTROL {
			chainControl = s.extractChainControlStatus(incident.DescriptionText)
		}

		// Update route status for closures
		if incident.FeedType == caltrans.LANE_CLOSURE && 
			 (incident.ParsedStatus == "closed" || incident.ParsedStatus == "closure") {
			routeStatus = api.RouteStatus_ROUTE_STATUS_RESTRICTED
		}
	}

	return routeStatus, chainControl, alerts, nil
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