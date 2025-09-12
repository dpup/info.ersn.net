package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
	
	"github.com/dpup/info.ersn.net/server/internal/cache"
	"github.com/dpup/info.ersn.net/server/internal/lib/alerts"
	"github.com/dpup/info.ersn.net/server/internal/lib/incident"
)

type MockIncident struct {
	ID          string
	Description string
	Location    string
	StyleUrl    string
	Latitude    float64
	Longitude   float64
	Category    string
}

func main() {
	// Get OpenAI API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Println("⚠️  OPENAI_API_KEY not set - using mock enhancer for demo")
		testWithMockEnhancer()
		return
	}
	
	log.Println("🔑 OpenAI API key found - testing real integration!")
	testWithRealOpenAI(apiKey)
}

func testWithMockEnhancer() {
	// Create components
	cacheInstance := cache.NewCache()
	store := cache.NewProcessedIncidentStore(cacheInstance)
	hasher := incident.NewIncidentContentHasher()
	
	// Use mock enhancer
	mockEnhancer := &mockAlertEnhancer{}
	enhancer := incident.NewAsyncAlertEnhancer(store, hasher, mockEnhancer)
	
	ctx := context.Background()
	
	// Test incident
	testIncident := MockIncident{
		ID:          "test-001",
		Description: "I-80 WESTBOUND CHAIN CONTROLS REQUIRED FROM DRUM TO NYACK",
		Location:    "I-80 Westbound at Drum",
		StyleUrl:    "#lcs",
		Latitude:    39.1234,
		Longitude:   -120.5678,
		Category:    "chain_control",
	}
	
	// Start workers
	enhancer.StartEnhancementWorkers(ctx)
	defer enhancer.StopEnhancementWorkers(ctx)
	
	// Queue for enhancement
	err := enhancer.QueueForEnhancement(ctx, testIncident)
	if err != nil {
		log.Fatal("Failed to queue for enhancement:", err)
	}
	
	// Wait a moment for processing
	log.Println("⏳ Waiting for background enhancement...")
	time.Sleep(500 * time.Millisecond)
	
	// Try to get enhanced result
	result, fromCache, err := enhancer.GetEnhancedAlert(ctx, testIncident)
	if err != nil {
		log.Fatal("Failed to get enhanced alert:", err)
	}
	
	fmt.Printf("✅ Enhancement complete!\n")
	fmt.Printf("From Cache: %v\n", fromCache)
	fmt.Printf("Result: %+v\n", result)
}

func testWithRealOpenAI(apiKey string) {
	// Create components
	cacheInstance := cache.NewCache()
	store := cache.NewProcessedIncidentStore(cacheInstance)
	hasher := incident.NewIncidentContentHasher()
	
	// Use real OpenAI enhancer
	realEnhancer := alerts.NewAlertEnhancer(apiKey, "gpt-4o-mini")
	enhancer := incident.NewAsyncAlertEnhancer(store, hasher, realEnhancer)
	
	ctx := context.Background()
	
	// Test incident
	testIncident := map[string]interface{}{
		"id":          "test-001",
		"description": "I-80 WESTBOUND CHAIN CONTROLS REQUIRED FROM DRUM TO NYACK",
		"location":    "I-80 Westbound at Drum",
		"style_url":   "#lcs",
		"category":    "chain_control",
	}
	
	// Start workers
	enhancer.StartEnhancementWorkers(ctx)
	defer enhancer.StopEnhancementWorkers(ctx)
	
	// Queue for enhancement
	err := enhancer.QueueForEnhancement(ctx, testIncident)
	if err != nil {
		log.Fatal("Failed to queue for enhancement:", err)
	}
	
	// Wait for processing
	log.Println("⏳ Waiting for real OpenAI enhancement...")
	time.Sleep(2 * time.Second) // Give more time for real API call
	
	// Try to get enhanced result
	result, fromCache, err := enhancer.GetEnhancedAlert(ctx, testIncident)
	if err != nil {
		log.Fatal("Failed to get enhanced alert:", err)
	}
	
	fmt.Printf("🚀 Real OpenAI Enhancement complete!\n")
	fmt.Printf("From Cache: %v\n", fromCache)
	fmt.Printf("Enhanced Result: %+v\n", result)
	
	// Show the difference
	if enhanced, ok := result.(alerts.EnhancedAlert); ok {
		fmt.Printf("\n📊 Enhancement Details:\n")
		fmt.Printf("Original: %s\n", enhanced.OriginalDescription)
		fmt.Printf("Enhanced: %s\n", enhanced.StructuredDescription.Details)
		fmt.Printf("Condensed: %s\n", enhanced.CondensedSummary)
		fmt.Printf("Impact: %s\n", enhanced.StructuredDescription.Impact)
		fmt.Printf("Duration: %s\n", enhanced.StructuredDescription.Duration)
	}
}

// mockAlertEnhancer for demo when no API key
type mockAlertEnhancer struct{}

func (m *mockAlertEnhancer) EnhanceAlert(ctx context.Context, raw alerts.RawAlert) (alerts.EnhancedAlert, error) {
	return alerts.EnhancedAlert{
		ID:                  raw.ID,
		OriginalDescription: raw.Description,
		StructuredDescription: alerts.StructuredDescription{
			Details: "Mock enhancement: " + raw.Description,
			Location: alerts.StructuredLocation{
				Description: raw.Location,
				Latitude:    39.1234,
				Longitude:   -120.5678,
			},
			Impact:           "moderate",
			Duration:         "< 1 hour",
			CondensedSummary: "Chain controls required, expect delays",
		},
		CondensedSummary: "Chain controls required, expect delays",
		ProcessedAt:      time.Now(),
	}, nil
}

func (m *mockAlertEnhancer) HealthCheck(ctx context.Context) error {
	return nil
}