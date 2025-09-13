package services

import (
	"context"
	"log"
	"time"
	
	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/config"
)

// PeriodicRefreshService simulates regular API requests to maintain cache warmth
// Replaces complex CacheWarmer with simple periodic calls to existing refresh logic
type PeriodicRefreshService struct {
	roadsService *RoadsService
	config       *config.Config
	
	// Background refresh control
	stopChan chan struct{}
	running  bool
}

// NewPeriodicRefreshService creates a new periodic refresh service
func NewPeriodicRefreshService(roadsService *RoadsService, config *config.Config) *PeriodicRefreshService {
	return &PeriodicRefreshService{
		roadsService: roadsService,
		config:       config,
		stopChan:     make(chan struct{}),
	}
}

// StartPeriodicRefresh begins simulated API requests to maintain cache freshness
// Uses existing refresh intervals from configuration
func (p *PeriodicRefreshService) StartPeriodicRefresh(ctx context.Context) error {
	if p.running {
		return nil // Already running
	}
	
	p.running = true
	
	// Use roads refresh interval from config (default 5 minutes)
	interval := p.config.Roads.RefreshInterval
	
	log.Printf("Starting periodic refresh every %v to maintain cache warmth", interval)
	
	// Start background goroutine for periodic refresh
	go p.refreshLoop(ctx, interval)
	
	return nil
}

// Stop gracefully stops the periodic refresh
func (p *PeriodicRefreshService) Stop() {
	if !p.running {
		return
	}
	
	p.running = false
	close(p.stopChan)
	log.Printf("Stopped periodic refresh service")
}

// refreshLoop runs the periodic refresh in background
func (p *PeriodicRefreshService) refreshLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	// Do initial refresh immediately
	p.simulateAPIRequest(ctx)
	
	for {
		select {
		case <-ctx.Done():
			log.Printf("Periodic refresh stopping due to context cancellation")
			return
		case <-p.stopChan:
			log.Printf("Periodic refresh stopping due to stop signal")
			return
		case <-ticker.C:
			p.simulateAPIRequest(ctx)
		}
	}
}

// simulateAPIRequest makes a simulated request to roads API to trigger cache refresh
// This leverages the existing refresh logic in RoadsService.ListRoads()
func (p *PeriodicRefreshService) simulateAPIRequest(ctx context.Context) {
	log.Printf("Periodic refresh: checking cache warmth")
	
	// Create a simulated request context with timeout
	refreshCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	
	// Call the existing ListRoads method which includes all the refresh logic
	// With the new stale-data serving logic, this will:
	// 1. Return stale data immediately if available
	// 2. Trigger background refresh if data is stale
	// 3. Only block if no data exists or data is very stale
	_, err := p.roadsService.ListRoads(refreshCtx, &api.ListRoadsRequest{})
	if err != nil {
		log.Printf("Periodic refresh failed: %v", err)
	} else {
		log.Printf("Periodic refresh: cache check completed")
	}
}

// IsRunning returns whether periodic refresh is active
func (p *PeriodicRefreshService) IsRunning() bool {
	return p.running
}