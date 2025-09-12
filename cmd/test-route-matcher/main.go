package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/dpup/info.ersn.net/server/internal/lib/geo"
	"github.com/dpup/info.ersn.net/server/internal/lib/routing"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "classify-alert":
		handleClassifyAlert()
	case "test-distance":
		handleTestDistance()
	case "validate-route":
		handleValidateRoute()
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func handleClassifyAlert() {
	fs := flag.NewFlagSet("classify-alert", flag.ExitOnError)
	alertFile := fs.String("alert-json", "", "Path to JSON file containing UnclassifiedAlert")
	routesFile := fs.String("routes-json", "", "Path to JSON file containing array of Routes")
	verbose := fs.Bool("verbose", false, "Show detailed classification process")

	fs.Parse(os.Args[2:])

	if *alertFile == "" || *routesFile == "" {
		fmt.Println("Example usage:")
		fmt.Println("  test-route-matcher classify-alert --alert-json alert.json --routes-json routes.json")
		fmt.Println("  test-route-matcher classify-alert --alert-json alert.json --routes-json routes.json --verbose")
		fmt.Println("")
		printSampleFiles()
		os.Exit(1)
	}

	// Read alert file
	alertData, err := os.ReadFile(*alertFile)
	if err != nil {
		log.Fatalf("Error reading alert file %s: %v", *alertFile, err)
	}

	var alert routing.UnclassifiedAlert
	if err := json.Unmarshal(alertData, &alert); err != nil {
		log.Fatalf("Error parsing alert JSON: %v", err)
	}

	// Read routes file
	routesData, err := os.ReadFile(*routesFile)
	if err != nil {
		log.Fatalf("Error reading routes file %s: %v", *routesFile, err)
	}

	var routes []routing.Route
	if err := json.Unmarshal(routesData, &routes); err != nil {
		log.Fatalf("Error parsing routes JSON: %v", err)
	}

	matcher := routing.NewRouteMatcher()
	ctx := context.Background()

	fmt.Printf("Classifying alert against %d route(s)...\n\n", len(routes))
	
	if *verbose {
		fmt.Printf("ALERT DETAILS:\n")
		fmt.Printf("  ID: %s\n", alert.ID)
		fmt.Printf("  Type: %s\n", alert.Type)
		fmt.Printf("  Description: %s\n", alert.Description)
		fmt.Printf("  Location: (%.6f, %.6f)\n", alert.Location.Latitude, alert.Location.Longitude)
		if alert.AffectedPolyline != nil {
			fmt.Printf("  Affected Polyline: %d points\n", len(alert.AffectedPolyline.Points))
		}
		fmt.Printf("\n")

		fmt.Printf("ROUTES:\n")
		for i, route := range routes {
			fmt.Printf("  Route %d:\n", i+1)
			fmt.Printf("    ID: %s\n", route.ID)
			fmt.Printf("    Name: %s\n", route.Name)
			fmt.Printf("    Section: %s\n", route.Section)
			fmt.Printf("    Points: %d\n", len(route.Polyline.Points))
			fmt.Printf("    Max Distance: %.0fm (%.1f miles)\n", route.MaxDistance, route.MaxDistance*0.000621371)
		}
		fmt.Printf("\n")
	}

	classified, err := matcher.ClassifyAlert(ctx, alert, routes)
	if err != nil {
		log.Fatalf("Error classifying alert: %v", err)
	}

	fmt.Printf("✅ Alert classified successfully!\n\n")
	
	fmt.Printf("CLASSIFICATION RESULT:\n")
	fmt.Printf("  Classification: %s\n", classified.Classification)
	fmt.Printf("  Distance to Route: %.2f meters (%.2f miles)\n", 
		classified.DistanceToRoute, classified.DistanceToRoute*0.000621371)
	fmt.Printf("  Affected Routes: %d\n", len(classified.RouteIDs))
	
	for i, routeID := range classified.RouteIDs {
		fmt.Printf("    %d. %s\n", i+1, routeID)
	}

	fmt.Printf("\nCLASSIFICATION MEANING:\n")
	switch classified.Classification {
	case routing.OnRoute:
		fmt.Printf("  🔴 ON_ROUTE: Alert directly affects route path\n")
		fmt.Printf("      - Distance < 100 meters from route polyline\n")
		fmt.Printf("      - High priority for travelers on affected routes\n")
	case routing.Nearby:
		fmt.Printf("  🟡 NEARBY: Alert in surrounding area but not blocking route\n")
		fmt.Printf("      - Distance within route threshold but > 100 meters\n")
		fmt.Printf("      - Medium priority, may cause delays or diversions\n")
	case routing.Distant:
		fmt.Printf("  🟢 DISTANT: Alert too far from monitored routes\n")
		fmt.Printf("      - Distance beyond route threshold\n")
		fmt.Printf("      - Low priority, unlikely to affect route travel\n")
	}
}

func handleTestDistance() {
	fs := flag.NewFlagSet("test-distance", flag.ExitOnError)
	routeID := fs.String("route-id", "", "Route ID to test against")
	lat := fs.Float64("lat", 0, "Latitude of test point")
	lng := fs.Float64("lng", 0, "Longitude of test point")
	routesFile := fs.String("routes-json", "", "Path to JSON file containing routes")

	fs.Parse(os.Args[2:])

	if *routeID == "" || (*lat == 0 && *lng == 0) {
		fmt.Println("Example usage:")
		fmt.Println("  test-route-matcher test-distance --route-id hwy4-angels-murphys --lat 38.1000 --lng -120.5000")
		fmt.Println("  test-route-matcher test-distance --route-id hwy4-angels-murphys --lat 38.1000 --lng -120.5000 --routes-json routes.json")
		os.Exit(1)
	}

	var routes []routing.Route

	if *routesFile != "" {
		// Read routes from file
		routesData, err := os.ReadFile(*routesFile)
		if err != nil {
			log.Fatalf("Error reading routes file: %v", err)
		}

		if err := json.Unmarshal(routesData, &routes); err != nil {
			log.Fatalf("Error parsing routes JSON: %v", err)
		}
	} else {
		// Create default Highway 4 route for testing
		routes = []routing.Route{
			{
				ID:      "hwy4-angels-murphys",
				Name:    "Hwy 4",
				Section: "Angels Camp to Murphys",
				Origin:  geo.Point{Latitude: 38.0675, Longitude: -120.5436},
				Destination: geo.Point{Latitude: 38.1391, Longitude: -120.4561},
				Polyline: geo.Polyline{
					Points: []geo.Point{
						{Latitude: 38.0675, Longitude: -120.5436}, // Angels Camp
						{Latitude: 38.1391, Longitude: -120.4561}, // Murphys
					},
				},
				MaxDistance: 16093.4, // 10 miles
			},
		}
	}

	// Find the specified route
	var targetRoute *routing.Route
	for _, route := range routes {
		if route.ID == *routeID {
			targetRoute = &route
			break
		}
	}

	if targetRoute == nil {
		log.Fatalf("Route ID %s not found in routes", *routeID)
	}

	testPoint := geo.Point{Latitude: *lat, Longitude: *lng}
	
	// Create a test alert
	testAlert := routing.UnclassifiedAlert{
		ID:          "distance-test",
		Location:    testPoint,
		Description: "Distance test alert",
		Type:        "test",
	}

	matcher := routing.NewRouteMatcher()
	ctx := context.Background()

	classified, err := matcher.ClassifyAlert(ctx, testAlert, []routing.Route{*targetRoute})
	if err != nil {
		log.Fatalf("Error testing distance: %v", err)
	}

	fmt.Printf("Distance test results:\n\n")
	fmt.Printf("ROUTE:\n")
	fmt.Printf("  ID: %s\n", targetRoute.ID)
	fmt.Printf("  Name: %s\n", targetRoute.Name)
	fmt.Printf("  Section: %s\n", targetRoute.Section)
	fmt.Printf("  Max Distance: %.0f meters (%.1f miles)\n", 
		targetRoute.MaxDistance, targetRoute.MaxDistance*0.000621371)

	fmt.Printf("\nTEST POINT:\n")
	fmt.Printf("  Coordinates: (%.6f, %.6f)\n", testPoint.Latitude, testPoint.Longitude)

	fmt.Printf("\nRESULT:\n")
	fmt.Printf("  Distance to Route: %.2f meters (%.2f km, %.2f miles)\n", 
		classified.DistanceToRoute, classified.DistanceToRoute/1000, classified.DistanceToRoute*0.000621371)
	fmt.Printf("  Classification: %s\n", classified.Classification)
	
	if len(classified.RouteIDs) > 0 {
		fmt.Printf("  Within Route Threshold: ✅ Yes\n")
	} else {
		fmt.Printf("  Within Route Threshold: ❌ No\n")
	}

	// Provide threshold information
	fmt.Printf("\nTHRESHOLD INFO:\n")
	fmt.Printf("  ON_ROUTE: < 100 meters\n")
	fmt.Printf("  NEARBY: < %.0f meters\n", targetRoute.MaxDistance)
	fmt.Printf("  DISTANT: > %.0f meters\n", targetRoute.MaxDistance)
}

func handleValidateRoute() {
	fs := flag.NewFlagSet("validate-route", flag.ExitOnError)
	routeFile := fs.String("route-json", "", "Path to JSON file containing Route")

	fs.Parse(os.Args[2:])

	if *routeFile == "" {
		fmt.Println("Example usage:")
		fmt.Println("  test-route-matcher validate-route --route-json route.json")
		fmt.Println("")
		printSampleRouteFile()
		os.Exit(1)
	}

	// Read route file
	routeData, err := os.ReadFile(*routeFile)
	if err != nil {
		log.Fatalf("Error reading route file %s: %v", *routeFile, err)
	}

	var route routing.Route
	if err := json.Unmarshal(routeData, &route); err != nil {
		log.Fatalf("Error parsing route JSON: %v", err)
	}

	fmt.Printf("Validating route...\n\n")

	// Basic validation
	errors := 0

	fmt.Printf("ROUTE DETAILS:\n")
	fmt.Printf("  ID: %s\n", route.ID)
	fmt.Printf("  Name: %s\n", route.Name)
	fmt.Printf("  Section: %s\n", route.Section)

	// Validate required fields
	if route.ID == "" {
		fmt.Printf("  ❌ ID is required\n")
		errors++
	} else {
		fmt.Printf("  ✅ ID is valid\n")
	}

	if route.Name == "" {
		fmt.Printf("  ❌ Name is required\n")
		errors++
	} else {
		fmt.Printf("  ✅ Name is valid\n")
	}

	// Validate polyline
	fmt.Printf("\nPOLYLINE VALIDATION:\n")
	if len(route.Polyline.Points) < 2 {
		fmt.Printf("  ❌ Polyline must have at least 2 points (found %d)\n", len(route.Polyline.Points))
		errors++
	} else {
		fmt.Printf("  ✅ Polyline has %d points\n", len(route.Polyline.Points))
		
		// Validate coordinates
		validCoords := 0
		for i, point := range route.Polyline.Points {
			if point.Latitude >= -90 && point.Latitude <= 90 &&
				point.Longitude >= -180 && point.Longitude <= 180 {
				validCoords++
			} else {
				fmt.Printf("  ❌ Point %d has invalid coordinates: (%.6f, %.6f)\n", 
					i+1, point.Latitude, point.Longitude)
				errors++
			}
		}
		
		if validCoords == len(route.Polyline.Points) {
			fmt.Printf("  ✅ All coordinates are valid\n")
		}

		// Calculate and display route length
		if len(route.Polyline.Points) >= 2 {
			geoUtils := geo.NewGeoUtils()
			totalDistance := 0.0
			
			for i := 0; i < len(route.Polyline.Points)-1; i++ {
				distance, err := geoUtils.PointToPoint(route.Polyline.Points[i], route.Polyline.Points[i+1])
				if err == nil {
					totalDistance += distance
				}
			}
			
			fmt.Printf("  ✅ Route length: %.2f km (%.2f miles)\n", 
				totalDistance/1000, totalDistance*0.000621371)
		}
	}

	// Validate thresholds
	fmt.Printf("\nTHRESHOLD VALIDATION:\n")
	if route.MaxDistance <= 0 {
		fmt.Printf("  ❌ MaxDistance must be positive (found %.2f)\n", route.MaxDistance)
		errors++
	} else {
		fmt.Printf("  ✅ MaxDistance: %.0f meters (%.1f miles)\n", 
			route.MaxDistance, route.MaxDistance*0.000621371)
	}

	// Summary
	fmt.Printf("\nVALIDATION SUMMARY:\n")
	if errors == 0 {
		fmt.Printf("  ✅ Route is valid and ready for use\n")
		fmt.Printf("  ✅ All required fields are present\n")
		fmt.Printf("  ✅ Polyline geometry is valid\n")
		fmt.Printf("  ✅ Thresholds are properly configured\n")
	} else {
		fmt.Printf("  ❌ Route has %d validation error(s)\n", errors)
		fmt.Printf("  ❌ Please fix the issues above before using this route\n")
		os.Exit(1)
	}
}

func printSampleFiles() {
	fmt.Println("Sample alert.json:")
	fmt.Println(`{
  "id": "test-001",
  "location": {"lat": 38.1000, "lng": -120.5000},
  "description": "Vehicle accident on Highway 4",
  "type": "incident"
}`)

	fmt.Println("")
	fmt.Println("Sample routes.json:")
	fmt.Println(`[
  {
    "id": "hwy4-angels-murphys",
    "name": "Hwy 4",
    "section": "Angels Camp to Murphys",
    "origin": {"lat": 38.0675, "lng": -120.5436},
    "destination": {"lat": 38.1391, "lng": -120.4561},
    "polyline": {
      "points": [
        {"lat": 38.0675, "lng": -120.5436},
        {"lat": 38.1391, "lng": -120.4561}
      ]
    },
    "max_distance": 16093.4
  }
]`)
}

func printSampleRouteFile() {
	fmt.Println("Sample route.json:")
	fmt.Println(`{`)
	fmt.Println(`  "id": "hwy4-angels-murphys",`)
	fmt.Println(`  "name": "Hwy 4",`)
	fmt.Println(`  "section": "Angels Camp to Murphys",`)
	fmt.Println(`  "origin": {"lat": 38.0675, "lng": -120.5436},`)
	fmt.Println(`  "destination": {"lat": 38.1391, "lng": -120.4561},`)
	fmt.Println(`  "polyline": {`)
	fmt.Println(`    "encoded_polyline": "sample_encoded_polyline",`)
	fmt.Println(`    "points": [`)
	fmt.Println(`      {"lat": 38.0675, "lng": -120.5436},`)
	fmt.Println(`      {"lat": 38.1391, "lng": -120.4561}`)
	fmt.Println(`    ]`)
	fmt.Println(`  },`)
	fmt.Println(`  "max_distance": 16093.4`)
	fmt.Println(`}`)
}

func printUsage() {
	fmt.Printf(`test-route-matcher - Route-aware alert classification testing tool

USAGE:
    test-route-matcher <command> [options]

COMMANDS:
    classify-alert      Classify alert against routes (ON_ROUTE/NEARBY/DISTANT)
    test-distance       Test distance calculation from point to specific route
    validate-route      Validate route JSON structure and geometry
    help               Show this help message

EXAMPLES:
    # Classify alert against routes
    test-route-matcher classify-alert --alert-json alert.json --routes-json routes.json --verbose
    
    # Test distance to Highway 4
    test-route-matcher test-distance --route-id hwy4-angels-murphys --lat 38.1000 --lng -120.5000
    
    # Validate route configuration
    test-route-matcher validate-route --route-json route.json

CLASSIFICATION TYPES:
    ON_ROUTE           Alert directly affects route path (< 100m from route)
    NEARBY             Alert in surrounding area (< route threshold, typically 10 miles)
    DISTANT            Alert too far from route (> route threshold)

For more information, visit: https://github.com/dpup/info.ersn.net
`)
}