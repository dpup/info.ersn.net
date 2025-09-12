package main

import (
	"fmt"
	"log"
	"time"

	"github.com/dpup/info.ersn.net/server/internal/cache"
	"github.com/dpup/info.ersn.net/server/internal/lib/alerts"
)

func main() {
	fmt.Println("Testing Simplified Content-Based Caching")
	
	// Initialize cache
	mainCache := cache.NewCache()
	cacheAdapter := cache.NewAlertCacheAdapter(mainCache)
	
	// Create content hasher
	hasher := alerts.NewContentHasher()
	
	// Test content hashing with similar incidents
	incident1 := alerts.RawAlert{
		ID:          "test1",
		Description: "Lane closure on Hwy 4 eastbound near Angels Camp",
		Location:    "Highway 4",
		Timestamp:   time.Now(),
	}
	
	incident2 := alerts.RawAlert{
		ID:          "test2", 
		Description: "LANE CLOSURE ON HWY 4 EASTBOUND NEAR ANGELS CAMP", // Same content, different case
		Location:    "Highway 4",
		Timestamp:   time.Now(),
	}
	
	incident3 := alerts.RawAlert{
		ID:          "test3",
		Description: "Construction on Highway 108 westbound",
		Location:    "Highway 108", 
		Timestamp:   time.Now(),
	}
	
	// Generate content hashes
	hash1 := hasher.HashRawAlert(incident1)
	hash2 := hasher.HashRawAlert(incident2)  
	hash3 := hasher.HashRawAlert(incident3)
	
	fmt.Printf("Incident 1 hash: %s\n", hash1[:8])
	fmt.Printf("Incident 2 hash: %s\n", hash2[:8])
	fmt.Printf("Incident 3 hash: %s\n", hash3[:8])
	
	// Check if similar incidents have same hash
	if hash1 == hash2 {
		fmt.Println("✅ Similar incidents correctly identified with same hash")
	} else {
		fmt.Println("❌ Similar incidents have different hashes")
	}
	
	if hash1 != hash3 {
		fmt.Println("✅ Different incidents correctly have different hashes")
	} else {
		fmt.Println("❌ Different incidents incorrectly have same hash")
	}
	
	// Test caching
	fmt.Println("\nTesting Enhanced Alert Caching:")
	
	// Create mock enhanced alert
	enhanced1 := alerts.EnhancedAlert{
		ID:                  incident1.ID,
		OriginalDescription: incident1.Description,
		StructuredDescription: alerts.StructuredDescription{
			Details: "Enhanced: Lane closure on Highway 4 eastbound near Angels Camp due to roadwork",
		},
		ProcessedAt: time.Now(),
	}
	
	// Cache the enhanced alert
	ttl := 24 * time.Hour
	err := cacheAdapter.SetEnhancedAlert(hash1, enhanced1, ttl)
	if err != nil {
		log.Printf("Failed to cache enhanced alert: %v", err)
	} else {
		fmt.Println("✅ Enhanced alert cached successfully")
	}
	
	// Try to retrieve cached alert using same content hash
	cachedAlert, found, err := cacheAdapter.GetEnhancedAlert(hash1)
	if err != nil {
		log.Printf("Error retrieving cached alert: %v", err)
	} else if found {
		fmt.Println("✅ Cached enhanced alert retrieved successfully")
		if enhanced, ok := cachedAlert.(alerts.EnhancedAlert); ok {
			fmt.Printf("   Enhanced description: %s\n", enhanced.StructuredDescription.Details)
		}
	} else {
		fmt.Println("❌ Cached alert not found")
	}
	
	// Try to retrieve using incident2's hash (should be same as incident1)
	_, found2, err := cacheAdapter.GetEnhancedAlert(hash2)
	if err != nil {
		log.Printf("Error retrieving cached alert with hash2: %v", err)
	} else if found2 {
		fmt.Println("✅ Content deduplication working - same content found with different incident")
	} else {
		fmt.Println("❌ Content deduplication failed - same content not found")
	}
	
	// Test cache check without retrieval
	if cacheAdapter.IsEnhancedAlertCached(hash3) {
		fmt.Println("❌ Uncached alert incorrectly reported as cached")
	} else {
		fmt.Println("✅ Uncached alert correctly identified as not cached")
	}
	
	fmt.Println("\n🎉 Simplified caching system test completed")
}