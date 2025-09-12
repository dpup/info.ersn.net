package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dpup/info.ersn.net/server/internal/cache"
	"github.com/dpup/info.ersn.net/server/internal/clients/caltrans"
	"github.com/dpup/info.ersn.net/server/internal/clients/google"
	"github.com/dpup/info.ersn.net/server/internal/config"
	"github.com/dpup/info.ersn.net/server/internal/lib/alerts"
	"github.com/dpup/info.ersn.net/server/internal/services"
	"github.com/dpup/info.ersn.net/server/internal/lib/routing"
	"github.com/dpup/info.ersn.net/server/internal/lib/geo"
)

// MockAlertEnhancer for testing caching without requiring OpenAI API key
type MockAlertEnhancer struct{}

func (m *MockAlertEnhancer) EnhanceAlert(ctx context.Context, raw alerts.RawAlert) (alerts.EnhancedAlert, error) {
	// Simulate OpenAI processing time
	time.Sleep(100 * time.Millisecond)
	
	return alerts.EnhancedAlert{
		OriginalDescription: raw.Description,
		CondensedSummary:    fmt.Sprintf("Enhanced: %s", raw.Description[:min(50, len(raw.Description))]),
		StructuredDescription: alerts.StructuredDescription{
			Details:      fmt.Sprintf("AI Enhanced: %s", raw.Description),
			Location:     alerts.StructuredLocation{Description: raw.Location},
			Impact:       "moderate",
			Duration:     "2-4 hours",
			TimeReported: time.Now().Format(time.RFC3339),
			AdditionalInfo: map[string]string{
				"enhanced_by": "mock_ai",
			},
		},
	}, nil
}

func (m *MockAlertEnhancer) HealthCheck(ctx context.Context) error {
	return nil
}

func main() {
	fmt.Println("Testing Integrated Alert Enhancement Caching...")
	
	// Create components
	cacheInstance := cache.NewCache()
	googleClient := google.NewClient("") // Empty key for test
	caltransClient := caltrans.NewFeedParser()
	
	// Use mock enhancer to avoid requiring API key
	mockEnhancer := &MockAlertEnhancer{}
	
	// Create minimal config
	roadsConfig := &config.RoadsConfig{}
	
	// Create RoadsService with integrated caching
	roadsService := services.NewRoadsService(
		googleClient, caltransClient, cacheInstance, roadsConfig, mockEnhancer,
	)
	
	ctx := context.Background()
	
	// Create test classified alert
	testAlert := routing.ClassifiedAlert{
		UnclassifiedAlert: routing.UnclassifiedAlert{
			ID:          "test-001",
			Title:       "Test Lane Closure",
			Description: "I-80 Westbound lane closure for maintenance work",
			Type:        "closure",
			Location: geo.Point{
				Latitude:  39.1234,
				Longitude: -120.5678,
			},
			StyleUrl: "#closure",
		},
		Classification: routing.OnRoute,
		RouteIDs:      []string{"test-route"},
	}
	
	// Test 1: First call should miss cache and call enhancer
	fmt.Println("\nTest 1: First enhancement call (cache miss)...")
	start := time.Now()
	enhanced1, err := roadsService.EnhanceAlertWithAI(ctx, testAlert)
	if err != nil {
		log.Fatalf("First enhancement failed: %v", err)
	}
	duration1 := time.Since(start)
	
	// Debug cache state
	keys := cacheInstance.Keys()
	fmt.Printf("Cache keys after first call: %v\n", keys)
	
	fmt.Printf("First call took: %v\n", duration1)
	fmt.Printf("Enhanced summary: %s\n", enhanced1.CondensedSummary)
	
	// Test 2: Second call with same content should hit cache
	fmt.Println("\nTest 2: Second enhancement call (cache hit)...")
	start = time.Now()
	enhanced2, err := roadsService.EnhanceAlertWithAI(ctx, testAlert)
	if err != nil {
		log.Fatalf("Second enhancement failed: %v", err)
	}
	duration2 := time.Since(start)
	
	fmt.Printf("Second call took: %v\n", duration2)
	fmt.Printf("Enhanced summary: %s\n", enhanced2.CondensedSummary)
	
	// Validate caching worked
	if duration2 >= duration1 {
		fmt.Printf("\n❌ FAIL: Second call should be much faster (cached)\n")
		fmt.Printf("First: %v, Second: %v\n", duration1, duration2)
		return
	}
	
	// Validate content is the same
	if enhanced1.CondensedSummary != enhanced2.CondensedSummary {
		fmt.Printf("\n❌ FAIL: Cached content doesn't match original\n")
		return
	}
	
	fmt.Printf("\n✅ SUCCESS: Integrated caching is working!\n")
	fmt.Printf("- First call: %v (cache miss + enhancement)\n", duration1)
	fmt.Printf("- Second call: %v (cache hit)\n", duration2)
	fmt.Printf("- Speedup: %.1fx faster\n", float64(duration1)/float64(duration2))
	
	// Test 3: Different content should miss cache
	testAlert2 := testAlert
	testAlert2.Description = "Different incident - I-5 Northbound accident"
	
	fmt.Println("\nTest 3: Different content (new cache miss)...")
	start = time.Now()
	enhanced3, err := roadsService.EnhanceAlertWithAI(ctx, testAlert2)
	if err != nil {
		log.Fatalf("Third enhancement failed: %v", err)
	}
	duration3 := time.Since(start)
	
	fmt.Printf("Third call took: %v\n", duration3)
	fmt.Printf("Enhanced summary: %s\n", enhanced3.CondensedSummary)
	
	if duration3 < 50*time.Millisecond {
		fmt.Printf("\n❌ FAIL: Different content should not be cached\n")
		return
	}
	
	fmt.Printf("\n✅ SUCCESS: Content-based caching correctly handles different inputs\n")
	
	// Show cache stats
	stats := cacheInstance.Stats()
	fmt.Printf("\nCache Statistics:\n")
	fmt.Printf("- Total entries: %d\n", stats.TotalEntries)
	fmt.Printf("- Fresh entries: %d\n", stats.FreshEntries)
	fmt.Printf("- Stale entries: %d\n", stats.StaleEntries)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}