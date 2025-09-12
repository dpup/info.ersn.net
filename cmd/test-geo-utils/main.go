package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/dpup/info.ersn.net/server/internal/lib/geo"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	geoUtils := geo.NewGeoUtils()

	switch command {
	case "point-distance":
		handlePointDistance(geoUtils)
	case "polyline-distance":
		handlePolylineDistance(geoUtils)
	case "polyline-overlap":
		handlePolylineOverlap(geoUtils)
	case "decode-polyline":
		handleDecodePolyline(geoUtils)
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func handlePointDistance(geoUtils geo.GeoUtils) {
	fs := flag.NewFlagSet("point-distance", flag.ExitOnError)
	lat1 := fs.Float64("lat1", 0, "Latitude of first point")
	lng1 := fs.Float64("lng1", 0, "Longitude of first point")
	lat2 := fs.Float64("lat2", 0, "Latitude of second point")
	lng2 := fs.Float64("lng2", 0, "Longitude of second point")

	fs.Parse(os.Args[2:])

	if *lat1 == 0 && *lng1 == 0 && *lat2 == 0 && *lng2 == 0 {
		fmt.Println("Example usage:")
		fmt.Println("  test-geo-utils point-distance --lat1 38.0675 --lng1 -120.5436 --lat2 38.1391 --lng2 -120.4561")
		fmt.Println("  (Distance between Angels Camp and Murphys)")
		os.Exit(1)
	}

	p1 := geo.Point{Latitude: *lat1, Longitude: *lng1}
	p2 := geo.Point{Latitude: *lat2, Longitude: *lng2}

	distance, err := geoUtils.PointToPoint(p1, p2)
	if err != nil {
		log.Fatalf("Error calculating distance: %v", err)
	}

	fmt.Printf("Distance between points:\n")
	fmt.Printf("  Point 1: (%.6f, %.6f)\n", p1.Latitude, p1.Longitude)
	fmt.Printf("  Point 2: (%.6f, %.6f)\n", p2.Latitude, p2.Longitude)
	fmt.Printf("  Distance: %.2f meters (%.2f km, %.2f miles)\n", 
		distance, distance/1000, distance*0.000621371)
}

func handlePolylineDistance(geoUtils geo.GeoUtils) {
	fs := flag.NewFlagSet("polyline-distance", flag.ExitOnError)
	lat := fs.Float64("lat", 0, "Latitude of point")
	lng := fs.Float64("lng", 0, "Longitude of point")
	polylineStr := fs.String("polyline", "", "Encoded polyline string")

	fs.Parse(os.Args[2:])

	if *lat == 0 && *lng == 0 && *polylineStr == "" {
		fmt.Println("Example usage:")
		fmt.Println("  test-geo-utils polyline-distance --lat 38.1000 --lng -120.5000 --polyline \"_p~iF~ps|U_ulLnnqC_mqNvxq`@\"")
		fmt.Println("  (Distance from point to Highway 4 route)")
		os.Exit(1)
	}

	point := geo.Point{Latitude: *lat, Longitude: *lng}
	
	// Create polyline from encoded string
	var polyline geo.Polyline
	if *polylineStr != "" {
		points, err := geoUtils.DecodePolyline(*polylineStr)
		if err != nil {
			log.Fatalf("Error decoding polyline: %v", err)
		}
		polyline = geo.Polyline{
			EncodedPolyline: *polylineStr,
			Points:          points,
		}
	} else {
		log.Fatal("Polyline is required")
	}

	distance, err := geoUtils.PointToPolyline(point, polyline)
	if err != nil {
		log.Fatalf("Error calculating distance to polyline: %v", err)
	}

	fmt.Printf("Distance from point to polyline:\n")
	fmt.Printf("  Point: (%.6f, %.6f)\n", point.Latitude, point.Longitude)
	fmt.Printf("  Polyline: %d points\n", len(polyline.Points))
	fmt.Printf("  Distance: %.2f meters (%.2f km, %.2f miles)\n", 
		distance, distance/1000, distance*0.000621371)
}

func handlePolylineOverlap(geoUtils geo.GeoUtils) {
	fs := flag.NewFlagSet("polyline-overlap", flag.ExitOnError)
	polyline1 := fs.String("polyline1", "", "First encoded polyline string")
	polyline2 := fs.String("polyline2", "", "Second encoded polyline string")
	threshold := fs.Float64("threshold", 50, "Threshold distance in meters")

	fs.Parse(os.Args[2:])

	if *polyline1 == "" && *polyline2 == "" {
		fmt.Println("Example usage:")
		fmt.Println("  test-geo-utils polyline-overlap --polyline1 \"route_polyline\" --polyline2 \"closure_polyline\" --threshold 50")
		fmt.Println("  (Check if closure overlaps with route within 50 meters)")
		os.Exit(1)
	}

	// Decode both polylines
	points1, err := geoUtils.DecodePolyline(*polyline1)
	if err != nil {
		log.Fatalf("Error decoding polyline1: %v", err)
	}

	points2, err := geoUtils.DecodePolyline(*polyline2)
	if err != nil {
		log.Fatalf("Error decoding polyline2: %v", err)
	}

	poly1 := geo.Polyline{EncodedPolyline: *polyline1, Points: points1}
	poly2 := geo.Polyline{EncodedPolyline: *polyline2, Points: points2}

	overlaps, segments, err := geoUtils.PolylinesOverlap(poly1, poly2, *threshold)
	if err != nil {
		log.Fatalf("Error checking polyline overlap: %v", err)
	}

	percentage, err := geoUtils.PolylineOverlapPercentage(poly1, poly2, *threshold)
	if err != nil {
		log.Fatalf("Error calculating overlap percentage: %v", err)
	}

	fmt.Printf("Polyline overlap analysis:\n")
	fmt.Printf("  Polyline 1: %d points\n", len(points1))
	fmt.Printf("  Polyline 2: %d points\n", len(points2))
	fmt.Printf("  Threshold: %.0f meters\n", *threshold)
	fmt.Printf("  Overlaps: %t\n", overlaps)
	fmt.Printf("  Overlap segments: %d\n", len(segments))
	fmt.Printf("  Overlap percentage: %.1f%%\n", percentage)
	
	if percentage > 10.0 {
		fmt.Printf("  Classification: ON_ROUTE (>10%% overlap)\n")
	} else if overlaps {
		fmt.Printf("  Classification: NEARBY (some overlap but <10%%)\n")
	} else {
		fmt.Printf("  Classification: DISTANT (no overlap within threshold)\n")
	}

	if len(segments) > 0 {
		fmt.Printf("  Overlap segments:\n")
		for i, segment := range segments {
			fmt.Printf("    %d: (%.6f, %.6f) to (%.6f, %.6f) - %.1fm\n",
				i+1, segment.StartPoint.Latitude, segment.StartPoint.Longitude,
				segment.EndPoint.Latitude, segment.EndPoint.Longitude, segment.Length)
		}
	}
}

func handleDecodePolyline(geoUtils geo.GeoUtils) {
	fs := flag.NewFlagSet("decode-polyline", flag.ExitOnError)
	polylineStr := fs.String("polyline", "", "Encoded polyline string to decode")
	verbose := fs.Bool("verbose", false, "Show all decoded points")

	fs.Parse(os.Args[2:])

	if *polylineStr == "" {
		fmt.Println("Example usage:")
		fmt.Println("  test-geo-utils decode-polyline --polyline \"_p~iF~ps|U_ulLnnqC_mqNvxq`@\"")
		fmt.Println("  test-geo-utils decode-polyline --polyline \"encoded_string\" --verbose")
		os.Exit(1)
	}

	points, err := geoUtils.DecodePolyline(*polylineStr)
	if err != nil {
		log.Fatalf("Error decoding polyline: %v", err)
	}

	fmt.Printf("Polyline decoded successfully:\n")
	fmt.Printf("  Input: %s\n", *polylineStr)
	fmt.Printf("  Points: %d\n", len(points))
	
	if len(points) > 0 {
		fmt.Printf("  Start: (%.6f, %.6f)\n", points[0].Latitude, points[0].Longitude)
		if len(points) > 1 {
			fmt.Printf("  End: (%.6f, %.6f)\n", points[len(points)-1].Latitude, points[len(points)-1].Longitude)
		}
	}

	if *verbose && len(points) > 0 {
		fmt.Printf("  All points:\n")
		for i, point := range points {
			fmt.Printf("    %d: (%.6f, %.6f)\n", i+1, point.Latitude, point.Longitude)
		}
	}
}

func printUsage() {
	fmt.Printf(`test-geo-utils - Geographic utility testing tool

USAGE:
    test-geo-utils <command> [options]

COMMANDS:
    point-distance      Calculate great-circle distance between two points
    polyline-distance   Calculate minimum distance from point to polyline
    polyline-overlap    Check if two polylines overlap within threshold
    decode-polyline     Decode Google polyline string to coordinates
    help               Show this help message

EXAMPLES:
    # Distance between Angels Camp and Murphys
    test-geo-utils point-distance --lat1 38.0675 --lng1 -120.5436 --lat2 38.1391 --lng2 -120.4561
    
    # Distance from point to Highway 4 route
    test-geo-utils polyline-distance --lat 38.1000 --lng -120.5000 --polyline "sample_polyline"
    
    # Check route vs closure overlap
    test-geo-utils polyline-overlap --polyline1 "route_encoded" --polyline2 "closure_encoded" --threshold 50
    
    # Decode polyline to see coordinates
    test-geo-utils decode-polyline --polyline "encoded_string" --verbose

For more information, visit: https://github.com/dpup/info.ersn.net
`)
}

// Helper function to parse coordinate pairs from string
func parseCoordinatePairs(coordStr string) ([]geo.Point, error) {
	if coordStr == "" {
		return nil, fmt.Errorf("empty coordinate string")
	}

	pairs := strings.Split(coordStr, ";")
	points := make([]geo.Point, 0, len(pairs))

	for _, pair := range pairs {
		coords := strings.Split(strings.TrimSpace(pair), ",")
		if len(coords) != 2 {
			return nil, fmt.Errorf("invalid coordinate pair: %s", pair)
		}

		lat, err := strconv.ParseFloat(strings.TrimSpace(coords[0]), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid latitude: %s", coords[0])
		}

		lng, err := strconv.ParseFloat(strings.TrimSpace(coords[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid longitude: %s", coords[1])
		}

		points = append(points, geo.Point{Latitude: lat, Longitude: lng})
	}

	return points, nil
}