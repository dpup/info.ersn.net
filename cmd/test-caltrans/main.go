package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/dpup/info.ersn.net/server/internal/clients/caltrans"
)

func main() {
	var (
		feedType = flag.String("feed", "all", "Feed type: all, chain, lanes, chp")
		lat      = flag.Float64("lat", 38.2, "Latitude for geographic filtering")
		lon      = flag.Float64("lon", -120.3, "Longitude for geographic filtering")
		radius   = flag.Float64("radius", 50000, "Radius in meters for geographic filtering")
		filter   = flag.Bool("filter", false, "Enable geographic filtering")
		help     = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		fmt.Printf("Caltrans KML Parser Test Tool\n\n")
		fmt.Printf("Tests the Caltrans KML feed parser implementation.\n\n")
		fmt.Printf("Usage: %s [options]\n\n", os.Args[0])
		fmt.Printf("Options:\n")
		flag.PrintDefaults()
		fmt.Printf("\nFeed types:\n")
		fmt.Printf("  all   - Test all feeds (default)\n")
		fmt.Printf("  chain - Chain control feed only\n")
		fmt.Printf("  lanes - Lane closures feed only\n")
		fmt.Printf("  chp   - CHP incidents feed only\n")
		fmt.Printf("\nExamples:\n")
		fmt.Printf("  %s\n", os.Args[0])
		fmt.Printf("  %s -feed=chain\n", os.Args[0])
		fmt.Printf("  %s -filter -lat=38.2 -lon=-120.3 -radius=25000\n", os.Args[0])
		return
	}

	fmt.Printf("Caltrans KML Parser Test\n")
	fmt.Printf("========================\n")
	fmt.Printf("Feed type: %s\n", *feedType)
	if *filter {
		fmt.Printf("Geographic filter: %.6f, %.6f (%.0f m radius)\n", *lat, *lon, *radius)
	}
	fmt.Printf("\n")

	// Create parser
	parser := caltrans.NewFeedParser()
	ctx := context.Background()

	switch *feedType {
	case "chain":
		testChainControls(parser, ctx)
	case "lanes":
		testLaneClosures(parser, ctx)
	case "chp":
		testCHPIncidents(parser, ctx)
	case "all":
		testChainControls(parser, ctx)
		testLaneClosures(parser, ctx)
		testCHPIncidents(parser, ctx)
		
		if *filter {
			testGeographicFiltering(parser, ctx, *lat, *lon, *radius)
		}
	default:
		log.Fatalf("Unknown feed type: %s", *feedType)
	}

	fmt.Printf("\n🎉 All Caltrans KML parser tests completed!\n")
}

func testChainControls(parser *caltrans.FeedParser, ctx context.Context) {
	fmt.Printf("Testing Chain Controls feed...\n")
	
	incidents, err := parser.ParseChainControls(ctx)
	if err != nil {
		log.Fatalf("ParseChainControls failed: %v", err)
	}

	fmt.Printf("✅ Chain Controls successful!\n")
	fmt.Printf("Incidents found: %d\n", len(incidents))
	
	if len(incidents) > 0 {
		printSampleIncident("Chain Control", incidents[0])
	}
	fmt.Printf("\n")
}

func testLaneClosures(parser *caltrans.FeedParser, ctx context.Context) {
	fmt.Printf("Testing Lane Closures feed...\n")
	
	incidents, err := parser.ParseLaneClosures(ctx)
	if err != nil {
		log.Fatalf("ParseLaneClosures failed: %v", err)
	}

	fmt.Printf("✅ Lane Closures successful!\n")
	fmt.Printf("Incidents found: %d\n", len(incidents))
	
	if len(incidents) > 0 {
		printSampleIncident("Lane Closure", incidents[0])
	}
	fmt.Printf("\n")
}

func testCHPIncidents(parser *caltrans.FeedParser, ctx context.Context) {
	fmt.Printf("Testing CHP Incidents feed...\n")
	
	incidents, err := parser.ParseCHPIncidents(ctx)
	if err != nil {
		log.Fatalf("ParseCHPIncidents failed: %v", err)
	}

	fmt.Printf("✅ CHP Incidents successful!\n")
	fmt.Printf("Incidents found: %d\n", len(incidents))
	
	if len(incidents) > 0 {
		printSampleIncident("CHP Incident", incidents[0])
	}
	fmt.Printf("\n")
}

func testGeographicFiltering(parser *caltrans.FeedParser, ctx context.Context, lat, lon, radius float64) {
	fmt.Printf("Testing Geographic Filtering...\n")
	
	routeCoords := []struct{ Lat, Lon float64 }{
		{lat, lon},
	}
	
	incidents, err := parser.ParseWithGeographicFilter(ctx, routeCoords, radius)
	if err != nil {
		log.Fatalf("ParseWithGeographicFilter failed: %v", err)
	}

	fmt.Printf("✅ Geographic Filtering successful!\n")
	fmt.Printf("Filtered incidents found: %d\n", len(incidents))
	
	if len(incidents) > 0 {
		printSampleIncident("Filtered Incident", incidents[0])
	}
	fmt.Printf("\n")
}

func printSampleIncident(label string, incident caltrans.CaltransIncident) {
	fmt.Printf("Sample %s:\n", label)
	fmt.Printf("  Name: %s\n", incident.Name)
	fmt.Printf("  Coordinates: %.6f, %.6f\n", 
		incident.Coordinates.Latitude, incident.Coordinates.Longitude)
	fmt.Printf("  Status: %s\n", incident.ParsedStatus)
	fmt.Printf("  Description: %s\n", truncateString(incident.DescriptionText, 100))
	if len(incident.ParsedDates) > 0 {
		fmt.Printf("  Dates: %v\n", incident.ParsedDates)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}