package main

import (
	"context"
	"fmt"
	"time"
	
	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/cache"
	"github.com/dpup/info.ersn.net/server/internal/clients/caltrans"
	"github.com/dpup/info.ersn.net/server/internal/clients/google"
	"github.com/dpup/info.ersn.net/server/internal/config"
	"github.com/dpup/info.ersn.net/server/internal/lib/alerts"
	"github.com/dpup/info.ersn.net/server/internal/services"
)

// nullAlertEnhancer for testing
type nullAlertEnhancer struct{}

func (n *nullAlertEnhancer) EnhanceAlert(ctx context.Context, raw alerts.RawAlert) (alerts.EnhancedAlert, error) {
	return alerts.EnhancedAlert{
		ID:                  raw.ID,
		OriginalDescription: raw.Description,
		StructuredDescription: alerts.StructuredDescription{
			Details:  "Test enhanced: " + raw.Description,
			Location: alerts.StructuredLocation{
				Description: raw.Location,
				Latitude:    0,
				Longitude:   0,
			},
			Impact:           "unknown",
			Duration:         "unknown",
			CondensedSummary: raw.Description,
		},
		CondensedSummary: raw.Description,
		ProcessedAt:      time.Now(),
	}, nil
}

func (n *nullAlertEnhancer) HealthCheck(ctx context.Context) error {
	return nil
}

func main() {
	fmt.Printf("Simplified Cache Refresh Test - Testing Periodic Refresh Strategy\n")
	fmt.Printf("================================================================\n")

	// Initialize components using the simplified approach
	cacheInstance := cache.NewCache()
	caltransClient := caltrans.NewFeedParser()
	googleClient := google.NewClient("") // Empty key for testing
	
	// Create mock config
	testConfig := &config.RoadsConfig{
		GoogleRoutes: config.GoogleConfig{
			RefreshInterval: 5 * time.Minute,
			APIKey:         "",
		},
		MonitoredRoads: []config.MonitoredRoad{
			{
				ID:   "test-route",
				Name: "Test Route",
				Origin: config.CoordinatesYAML{
					Latitude:  38.067400,
					Longitude: -120.540200,
				},
				Destination: config.CoordinatesYAML{
					Latitude:  38.139117,
					Longitude: -120.456111,
				},
			},
		},
	}
	
	// Create alert enhancer with caching
	baseEnhancer := &nullAlertEnhancer{}
	cacheAdapter := cache.NewAlertCacheAdapter(cacheInstance)
	cachedEnhancer := alerts.NewCachedAlertEnhancer(baseEnhancer, cacheAdapter)
	
	// Initialize roads service with simplified approach
	roadsService := services.NewRoadsService(googleClient, caltransClient, cacheInstance, testConfig, cachedEnhancer)
	
	// Create periodic refresh service (replaces cache warmer)
	periodicRefresh := services.NewPeriodicRefreshService(roadsService, testConfig)
	
	fmt.Printf("Testing simplified cache refresh approach...\n\n")
	
	// Test single refresh call (simulates what periodic service does)
	fmt.Printf("1. Testing single simulated API call...\n")
	startTime := time.Now()
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	// This simulates what the periodic refresh service does
	_, err := roadsService.ListRoads(ctx, &api.ListRoadsRequest{})
	if err != nil {
		fmt.Printf("   ❌ Simulated API call failed: %v\n", err)
	} else {
		duration := time.Since(startTime)
		fmt.Printf("   ✅ Simulated API call completed in %v\n", duration)
	}
	
	// Show cache statistics
	stats := cacheInstance.Stats()
	fmt.Printf("\n2. Cache Statistics after refresh:\n")
	fmt.Printf("   Total entries: %d\n", stats.TotalEntries)
	fmt.Printf("   Fresh entries: %d\n", stats.FreshEntries)
	fmt.Printf("   Stale entries: %d\n", stats.StaleEntries)
	
	// Test content-based caching
	fmt.Printf("\n3. Testing content-based alert caching:\n")
	
	testAlert := alerts.RawAlert{
		ID:          "test123",
		Description: "Test incident on Highway 4",
		Location:    "Highway 4",
		Timestamp:   time.Now(),
	}
	
	// Check if enhancement would be cached
	if cachedEnhancer.IsAlertCached(testAlert) {
		fmt.Printf("   ✅ Alert would be served from cache\n")
	} else {
		fmt.Printf("   📋 Alert would trigger OpenAI processing (normal for first time)\n")
	}
	
	// Test periodic refresh service
	fmt.Printf("\n4. Testing periodic refresh service:\n")
	fmt.Printf("   Starting periodic refresh (will run in background)...\n")
	
	if err := periodicRefresh.StartPeriodicRefresh(ctx); err != nil {
		fmt.Printf("   ❌ Failed to start periodic refresh: %v\n", err)
	} else {
		fmt.Printf("   ✅ Periodic refresh service started\n")
		fmt.Printf("   🔄 Service will refresh every %v\n", testConfig.GoogleRoutes.RefreshInterval)
	}
	
	// Wait a bit to show it's working
	fmt.Printf("\n   Waiting 10 seconds to demonstrate periodic refresh...\n")
	time.Sleep(10 * time.Second)
	
	periodicRefresh.Stop()
	fmt.Printf("   🛑 Periodic refresh service stopped\n")
	
	fmt.Printf("\n🎉 Simplified cache refresh test completed!\n")
	fmt.Printf("\nKey improvements over old system:\n")
	fmt.Printf("• No complex background workers\n")
	fmt.Printf("• No dedicated cache warming service\n") 
	fmt.Printf("• Uses existing refresh logic in RoadsService\n")
	fmt.Printf("• Content-based deduplication prevents redundant OpenAI calls\n")
	fmt.Printf("• Simpler architecture with fewer moving parts\n")
}