package main

import (
	"context"
	"fmt"
	"log"
	"time"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/clients/caltrans"
	"github.com/dpup/info.ersn.net/server/internal/lib/incident"
)

func main() {
	fmt.Println("🧪 Testing incident hashing with CaltransIncident structs...")

	hasher := incident.NewIncidentContentHasher()
	ctx := context.Background()

	// Create a sample CaltransIncident
	sampleIncident := caltrans.CaltransIncident{
		FeedType:        caltrans.LANE_CLOSURE,
		Name:            "Lane Closure on I-80",
		DescriptionHtml: "<p>I-80 WESTBOUND - LANE CLOSURE - near Drum</p>",
		DescriptionText: "I-80 WESTBOUND - LANE CLOSURE - near Drum",
		StyleUrl:        "#lcs",
		Coordinates: &api.Coordinates{
			Latitude:  39.1234,
			Longitude: -120.5678,
		},
		ParsedStatus: "Active",
		LastFetched:  time.Now(),
	}

	fmt.Printf("📋 Sample Incident:\n")
	fmt.Printf("  - Description: %s\n", sampleIncident.DescriptionText)
	fmt.Printf("  - Feed Type: %d (lane_closure)\n", sampleIncident.FeedType)
	fmt.Printf("  - Coordinates: %.4f, %.4f\n", 
		sampleIncident.Coordinates.Latitude, 
		sampleIncident.Coordinates.Longitude)

	// Test hashing
	hash, err := hasher.HashIncident(ctx, sampleIncident)
	if err != nil {
		log.Fatalf("❌ Failed to hash incident: %v", err)
	}

	fmt.Printf("✅ Incident hashed successfully!\n")
	fmt.Printf("📊 Hash Details:\n")
	fmt.Printf("  - Content Hash: %s\n", hash.ContentHash[:16]+"...")
	fmt.Printf("  - Normalized Text: %s\n", hash.NormalizedText)
	fmt.Printf("  - Location Key: %s\n", hash.LocationKey)
	fmt.Printf("  - Category: %s\n", hash.IncidentCategory)
	fmt.Printf("  - First Seen: %v\n", hash.FirstSeenAt.Format("2006-01-02 15:04:05"))

	// Test with different incident types
	fmt.Printf("\n🔄 Testing different feed types...\n")
	
	feedTypes := []struct {
		feedType caltrans.CaltransFeedType
		name     string
		expected string
	}{
		{caltrans.CHAIN_CONTROL, "Chain Control", "chain_control"},
		{caltrans.LANE_CLOSURE, "Lane Closure", "lane_closure"},
		{caltrans.CHP_INCIDENT, "CHP Incident", "chp_incident"},
	}

	for _, ft := range feedTypes {
		incident := caltrans.CaltransIncident{
			FeedType:        ft.feedType,
			DescriptionText: fmt.Sprintf("Test %s incident", ft.name),
			Coordinates: &api.Coordinates{
				Latitude:  37.7749,
				Longitude: -122.4194,
			},
		}

		hash, err := hasher.HashIncident(ctx, incident)
		if err != nil {
			log.Printf("❌ Failed to hash %s: %v", ft.name, err)
			continue
		}

		if hash.IncidentCategory == ft.expected {
			fmt.Printf("  ✅ %s → %s\n", ft.name, hash.IncidentCategory)
		} else {
			fmt.Printf("  ❌ %s → %s (expected %s)\n", ft.name, hash.IncidentCategory, ft.expected)
		}
	}

	fmt.Println("\n🎉 Incident hashing test completed!")
}