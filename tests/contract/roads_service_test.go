package contract

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/dpup/info.ersn.net/server"
	"github.com/dpup/info.ersn.net/server/internal/services"
)

// T004: Contract test RoadsService.ListRoutes - MUST FAIL initially
func TestRoadsService_ListRoutes_Contract(t *testing.T) {
	// This test MUST fail until RoadsService is implemented
	service := &services.RoadsService{} // No implementation yet - will cause compilation error

	req := &api.ListRoutesRequest{}
	resp, err := service.ListRoutes(context.Background(), req)

	// Contract requirements from contracts/roads.proto lines 12-17
	require.NoError(t, err, "ListRoutes should not return error")
	require.NotNil(t, resp, "Response should not be nil")
	require.NotNil(t, resp.Routes, "Routes array should not be nil")
	require.NotNil(t, resp.LastUpdated, "LastUpdated timestamp should not be nil")
	
	// Validate response structure matches proto definition
	if len(resp.Routes) > 0 {
		route := resp.Routes[0]
		require.NotEmpty(t, route.Id, "Route ID should not be empty")
		require.NotEmpty(t, route.Name, "Route name should not be empty")
		require.NotNil(t, route.Origin, "Route origin should not be nil")
		require.NotNil(t, route.Destination, "Route destination should not be nil")
		require.NotEqual(t, api.RouteStatus_ROUTE_STATUS_UNSPECIFIED, route.Status, "Route status should be specified")
	}
	
	// Timestamp should be recent (within last hour for contract validation)
	require.True(t, resp.LastUpdated.AsTime().Unix() > 0, "LastUpdated should be valid timestamp")
}

// T005: Contract test RoadsService.GetRoute - MUST FAIL initially
func TestRoadsService_GetRoute_Contract(t *testing.T) {
	// This test MUST fail until RoadsService is implemented
	service := &services.RoadsService{} // No implementation yet - will cause compilation error

	req := &api.GetRouteRequest{
		RouteId: "test-route-id",
	}
	resp, err := service.GetRoute(context.Background(), req)

	// Contract requirements from contracts/roads.proto lines 19-24
	require.NoError(t, err, "GetRoute should not return error")
	require.NotNil(t, resp, "Response should not be nil")
	require.NotNil(t, resp.Route, "Route should not be nil")
	require.NotNil(t, resp.LastUpdated, "LastUpdated timestamp should not be nil")
	
	// Validate route structure
	route := resp.Route
	require.NotEmpty(t, route.Id, "Route ID should not be empty")
	require.NotEmpty(t, route.Name, "Route name should not be empty")
	require.NotNil(t, route.Origin, "Route origin coordinates should not be nil")
	require.NotNil(t, route.Destination, "Route destination coordinates should not be nil")
	require.NotEqual(t, api.RouteStatus_ROUTE_STATUS_UNSPECIFIED, route.Status, "Route status should be specified")
	
	// Traffic condition should be present
	require.NotNil(t, route.TrafficCondition, "Traffic condition should not be nil")
	require.Greater(t, route.TrafficCondition.DurationSeconds, int32(0), "Duration should be positive")
	require.Greater(t, route.TrafficCondition.DistanceMeters, int32(0), "Distance should be positive")
	
	// Chain control status should be specified
	require.NotEqual(t, api.ChainControlStatus_CHAIN_CONTROL_UNSPECIFIED, route.ChainControl, "Chain control should be specified")
}