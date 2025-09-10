package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	api "github.com/dpup/info.ersn.net/server"
	"github.com/dpup/info.ersn.net/server/internal/clients/google"
)

func main() {
	var (
		apiKey    = flag.String("api-key", "", "Google Routes API key (or set GOOGLE_ROUTES_API_KEY env var)")
		originStr = flag.String("origin", "38.067400,-120.540200", "Origin coordinates (lat,lon)")
		destStr   = flag.String("dest", "38.139117,-120.456111", "Destination coordinates (lat,lon)")
		help      = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		fmt.Printf("Google Routes API Test Tool\n\n")
		fmt.Printf("Tests the Google Routes API client implementation.\n\n")
		fmt.Printf("Usage: %s [options]\n\n", os.Args[0])
		fmt.Printf("Options:\n")
		flag.PrintDefaults()
		fmt.Printf("\nExamples:\n")
		fmt.Printf("  %s -api-key=YOUR_KEY\n", os.Args[0])
		fmt.Printf("  %s -origin=\"37.7749,-122.4194\" -dest=\"34.0522,-118.2437\"\n", os.Args[0])
		fmt.Printf("  GOOGLE_ROUTES_API_KEY=your_key %s\n", os.Args[0])
		return
	}

	// Get API key from flag or environment
	key := *apiKey
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
		if key == "" {
			key = os.Getenv("GOOGLE_ROUTES_API_KEY") // fallback
		}
	}
	if key == "" {
		log.Fatal("Google Routes API key required. Use -api-key flag or GOOGLE_API_KEY/GOOGLE_ROUTES_API_KEY env var")
	}

	// Parse coordinates
	var originLat, originLon, destLat, destLon float64
	_, err := fmt.Sscanf(*originStr, "%f,%f", &originLat, &originLon)
	if err != nil {
		log.Fatalf("Invalid origin coordinates: %v", err)
	}
	
	_, err = fmt.Sscanf(*destStr, "%f,%f", &destLat, &destLon)
	if err != nil {
		log.Fatalf("Invalid destination coordinates: %v", err)
	}

	fmt.Printf("Google Routes API Test\n")
	fmt.Printf("======================\n")
	fmt.Printf("Origin: %.6f, %.6f\n", originLat, originLon)
	fmt.Printf("Destination: %.6f, %.6f\n", destLat, destLon)
	fmt.Printf("API Key: %s...\n", key[:min(len(key), 10)])
	fmt.Printf("\n")

	// Create client and test
	client := google.NewClient(key)
	
	// Create coordinate structures
	origin := &api.Coordinates{
		Latitude:  originLat,
		Longitude: originLon,
	}
	destination := &api.Coordinates{
		Latitude:  destLat,
		Longitude: destLon,
	}
	
	fmt.Printf("Testing ComputeRoutes...\n")
	route, err := client.ComputeRoutes(context.Background(), origin, destination)
	if err != nil {
		log.Fatalf("ComputeRoutes failed: %v", err)
	}

	fmt.Printf("✅ ComputeRoutes successful!\n")
	fmt.Printf("Distance: %.2f km\n", float64(route.DistanceMeters)/1000.0)
	fmt.Printf("Duration: %.1f minutes\n", float64(route.DurationSeconds)/60.0)
	fmt.Printf("Polyline: %s...\n", route.Polyline[:min(len(route.Polyline), 50)])
	
	if len(route.SpeedReadings) > 0 {
		fmt.Printf("Speed readings: %d\n", len(route.SpeedReadings))
		fmt.Printf("Traffic conditions found:\n")
		conditions := make(map[string]int)
		for _, reading := range route.SpeedReadings {
			conditions[reading.SpeedCategory]++
		}
		for category, count := range conditions {
			fmt.Printf("  %s: %d segments\n", category, count)
		}
	}

	fmt.Printf("\n🎉 All Google Routes API tests passed!\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}