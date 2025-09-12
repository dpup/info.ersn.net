package services

import (
	"context"
	"log"
	"time"
	
	"github.com/dpup/info.ersn.net/server/internal/clients/caltrans"
	"github.com/dpup/info.ersn.net/server/internal/config"
	"github.com/dpup/info.ersn.net/server/internal/lib/incident"
)

// CacheWarmer fetches current feed data and processes it for cache warming
type CacheWarmer struct {
	caltransClient *caltrans.FeedParser
	processor      incident.IncidentBatchProcessor
	roadsConfig    *config.RoadsConfig
}

// NewCacheWarmer creates a new cache warming service
func NewCacheWarmer(caltransClient *caltrans.FeedParser, processor incident.IncidentBatchProcessor, roadsConfig *config.RoadsConfig) *CacheWarmer {
	return &CacheWarmer{
		caltransClient: caltransClient,
		processor:      processor,
		roadsConfig:    roadsConfig,
	}
}

// WarmCache fetches current incidents from feeds and processes only route-relevant ones
func (w *CacheWarmer) WarmCache(ctx context.Context) error {
	log.Printf("Starting cache warming with route-filtered current feed data...")
	
	// Create a context with timeout for the warming process
	warmCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	
	// Extract route coordinates from configuration
	var routeCoordinates []struct{ Lat, Lon float64 }
	for _, route := range w.roadsConfig.MonitoredRoads {
		routeCoordinates = append(routeCoordinates, struct{ Lat, Lon float64 }{
			Lat: route.Origin.Latitude,
			Lon: route.Origin.Longitude,
		})
		routeCoordinates = append(routeCoordinates, struct{ Lat, Lon float64 }{
			Lat: route.Destination.Latitude, 
			Lon: route.Destination.Longitude,
		})
	}
	
	if len(routeCoordinates) == 0 {
		log.Printf("No monitored routes configured - skipping cache warming")
		return nil
	}
	
	log.Printf("Cache warming: filtering incidents within 10km of %d route points", len(routeCoordinates))
	
	// Use geographic filtering to get only route-relevant incidents
	filteredIncidents, err := w.caltransClient.ParseWithGeographicFilter(warmCtx, routeCoordinates, 10000) // 10km radius
	if err != nil {
		return err
	}
	
	log.Printf("Cache warming: found %d route-relevant incidents (after geographic filtering)", len(filteredIncidents))
	
	// Convert to interface{} slice for processing
	var allIncidents []interface{}
	for _, incident := range filteredIncidents {
		allIncidents = append(allIncidents, incident)
	}
	
	// Start background processing if not already running
	if err := w.processor.StartBackgroundProcessing(warmCtx); err != nil {
		log.Printf("Background processing already running: %v", err)
	}
	
	// Process the current feed data through the background processor
	if err := w.processor.WarmCacheWithCurrentFeed(warmCtx, allIncidents); err != nil {
		return err
	}
	
	// Give some time for initial processing to complete
	log.Printf("Cache warming initiated - allowing 30 seconds for initial processing...")
	select {
	case <-time.After(30 * time.Second):
		log.Printf("Cache warming phase complete")
	case <-warmCtx.Done():
		log.Printf("Cache warming interrupted: %v", warmCtx.Err())
		return warmCtx.Err()
	}
	
	return nil
}