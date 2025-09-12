package incident

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// asyncAlertEnhancer implements AsyncAlertEnhancer interface
type asyncAlertEnhancer struct {
	store   ProcessedIncidentStore
	hasher  IncidentContentHasher
	
	// Enhancement queue for background processing
	enhancementQueue chan interface{}
	processing       bool
	processingMu     sync.RWMutex
	
	// Metrics and statistics
	stats EnhancementStatus
	statsMu sync.RWMutex
	
	// Configuration
	maxConcurrent int
	timeout       time.Duration
}

// NewAsyncAlertEnhancer creates a new async alert enhancer
func NewAsyncAlertEnhancer(store ProcessedIncidentStore, hasher IncidentContentHasher) AsyncAlertEnhancer {
	enhancer := &asyncAlertEnhancer{
		store:            store,
		hasher:           hasher,
		enhancementQueue: make(chan interface{}, 500), // Buffer for 500 enhancements
		maxConcurrent:    3,                           // Limit OpenAI concurrent requests
		timeout:          45 * time.Second,            // OpenAI timeout
	}
	
	// Initialize stats
	enhancer.stats = EnhancementStatus{
		CachedEnhancementsAvailable: 0,
		CacheHitRateLast24h:        0.0,
		BackgroundQueueSize:        0,
		ResponseTimeP95:            0.0,
		IsHealthy:                  true,
	}
	
	return enhancer
}

// GetEnhancedAlert returns enhanced alert immediately from cache if available
func (e *asyncAlertEnhancer) GetEnhancedAlert(ctx context.Context, incident interface{}) (interface{}, bool, error) {
	startTime := time.Now()
	
	// Generate content hash for incident
	hash, err := e.hasher.HashIncident(ctx, incident)
	if err != nil {
		return nil, false, fmt.Errorf("failed to hash incident: %w", err)
	}
	
	// Try to get enhanced version from cache
	cached, err := e.store.GetProcessed(ctx, hash, OPENAI_ENHANCED)
	if err != nil {
		log.Printf("Failed to get enhanced alert from cache: %v", err)
		// Fall back to raw incident
		return incident, false, nil
	}
	
	if cached != nil {
		// Cache hit - return enhanced data
		e.updateResponseTimeStats(time.Since(startTime), true)
		e.incrementCacheHit()
		
		// Update serve count
		cached.ServeCount++
		if err := e.store.StoreProcessed(ctx, *cached); err != nil {
			log.Printf("Failed to update serve count: %v", err)
		}
		
		return cached.ProcessedData, true, nil
	}
	
	// Cache miss - queue for background enhancement and return raw data
	if err := e.QueueForEnhancement(ctx, incident); err != nil {
		log.Printf("Failed to queue incident for enhancement: %v", err)
	}
	
	e.updateResponseTimeStats(time.Since(startTime), false)
	e.incrementCacheMiss()
	
	return incident, false, nil
}

// QueueForEnhancement adds incident to background processing queue
func (e *asyncAlertEnhancer) QueueForEnhancement(ctx context.Context, incident interface{}) error {
	select {
	case e.enhancementQueue <- incident:
		e.incrementQueueSize()
		return nil
	default:
		// Queue is full
		return fmt.Errorf("enhancement queue is full")
	}
}

// GetEnhancementStatus returns cache hit rate and queue statistics
func (e *asyncAlertEnhancer) GetEnhancementStatus(ctx context.Context) (EnhancementStatus, error) {
	e.statsMu.RLock()
	defer e.statsMu.RUnlock()
	
	// Update queue size
	status := e.stats
	status.BackgroundQueueSize = int64(len(e.enhancementQueue))
	
	// Update cached enhancements available
	// This would typically query the store for count of OPENAI_ENHANCED entries
	// For now, we'll use a placeholder implementation
	status.CachedEnhancementsAvailable = 100 // Placeholder
	
	// Health check: system is healthy if response time is under 200ms
	status.IsHealthy = status.ResponseTimeP95 < 200.0
	
	return status, nil
}

// StartEnhancementWorkers starts the background enhancement processing
func (e *asyncAlertEnhancer) StartEnhancementWorkers(ctx context.Context) error {
	e.processingMu.Lock()
	defer e.processingMu.Unlock()
	
	if e.processing {
		return fmt.Errorf("enhancement workers already running")
	}
	
	e.processing = true
	
	// Start worker goroutines for OpenAI enhancement
	for i := 0; i < e.maxConcurrent; i++ {
		go e.enhancementWorker(ctx, i)
	}
	
	log.Printf("Started %d OpenAI enhancement workers", e.maxConcurrent)
	return nil
}

// StopEnhancementWorkers gracefully stops background processing
func (e *asyncAlertEnhancer) StopEnhancementWorkers(ctx context.Context) error {
	e.processingMu.Lock()
	defer e.processingMu.Unlock()
	
	if !e.processing {
		return nil
	}
	
	e.processing = false
	close(e.enhancementQueue)
	
	log.Printf("Stopped OpenAI enhancement workers")
	return nil
}

// enhancementWorker processes incidents from the enhancement queue
func (e *asyncAlertEnhancer) enhancementWorker(ctx context.Context, workerID int) {
	log.Printf("OpenAI enhancement worker %d started", workerID)
	
	for {
		select {
		case <-ctx.Done():
			log.Printf("Enhancement worker %d stopping due to context cancellation", workerID)
			return
		case incident, ok := <-e.enhancementQueue:
			if !ok {
				log.Printf("Enhancement worker %d stopping due to queue closure", workerID)
				return
			}
			
			// Process the enhancement
			e.enhanceIncident(ctx, incident, workerID)
			e.decrementQueueSize()
		}
	}
}

// enhanceIncident performs OpenAI enhancement on a single incident
func (e *asyncAlertEnhancer) enhanceIncident(ctx context.Context, incident interface{}, workerID int) {
	startTime := time.Now()
	
	// Generate content hash
	hash, err := e.hasher.HashIncident(ctx, incident)
	if err != nil {
		log.Printf("Enhancement worker %d: Failed to hash incident: %v", workerID, err)
		return
	}
	
	// Create processing context with timeout
	processCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	
	// TODO: This is where we would call the actual OpenAI API
	// For now, we'll simulate the enhancement process
	enhancedData := e.simulateOpenAIEnhancement(incident)
	
	// Store the enhanced result
	enhancedEntry := ProcessedIncidentCache{
		ContentHash:        hash,
		Stage:             OPENAI_ENHANCED,
		OriginalIncident:  incident,
		ProcessedData:     enhancedData,
		LastSeenInFeed:    time.Now(),
		CacheExpiresAt:    time.Now().Add(24 * time.Hour), // 24 hour cache for enhanced data
		ServeCount:        0,
		ProcessingDuration: time.Since(startTime),
	}
	
	if err := e.store.StoreProcessed(processCtx, enhancedEntry); err != nil {
		log.Printf("Enhancement worker %d: Failed to store enhanced incident: %v", workerID, err)
		return
	}
	
	processingDuration := time.Since(startTime)
	log.Printf("Enhancement worker %d: Enhanced incident %s in %v", workerID, hash.ContentHash[:8], processingDuration)
	
	// Update statistics
	e.incrementEnhancedCount()
}

// simulateOpenAIEnhancement simulates OpenAI processing
// TODO: Replace with actual OpenAI API integration
func (e *asyncAlertEnhancer) simulateOpenAIEnhancement(incident interface{}) interface{} {
	// Simulate processing delay
	time.Sleep(100 * time.Millisecond)
	
	// Return enhanced version (for now, just add a field indicating enhancement)
	switch v := incident.(type) {
	case map[string]interface{}:
		enhanced := make(map[string]interface{})
		for k, val := range v {
			enhanced[k] = val
		}
		enhanced["openai_enhanced"] = true
		enhanced["enhancement_timestamp"] = time.Now().Format(time.RFC3339)
		enhanced["enhanced_description"] = fmt.Sprintf("Enhanced: %v", v["description"])
		return enhanced
	default:
		// For struct types or other types, return wrapped version
		return map[string]interface{}{
			"original_incident":     incident,
			"openai_enhanced":      true,
			"enhancement_timestamp": time.Now().Format(time.RFC3339),
			"enhanced_description":  "Enhanced incident data (processed by OpenAI)",
		}
	}
}

// Statistics update methods

func (e *asyncAlertEnhancer) incrementCacheHit() {
	e.statsMu.Lock()
	defer e.statsMu.Unlock()
	// Update cache hit rate calculation
	// This would typically maintain a sliding window of requests
}

func (e *asyncAlertEnhancer) incrementCacheMiss() {
	e.statsMu.Lock()
	defer e.statsMu.Unlock()
	// Update cache miss statistics
}

func (e *asyncAlertEnhancer) incrementQueueSize() {
	e.statsMu.Lock()
	defer e.statsMu.Unlock()
	e.stats.BackgroundQueueSize++
}

func (e *asyncAlertEnhancer) decrementQueueSize() {
	e.statsMu.Lock()
	defer e.statsMu.Unlock()
	if e.stats.BackgroundQueueSize > 0 {
		e.stats.BackgroundQueueSize--
	}
}

func (e *asyncAlertEnhancer) incrementEnhancedCount() {
	e.statsMu.Lock()
	defer e.statsMu.Unlock()
	e.stats.CachedEnhancementsAvailable++
}

func (e *asyncAlertEnhancer) updateResponseTimeStats(duration time.Duration, cacheHit bool) {
	e.statsMu.Lock()
	defer e.statsMu.Unlock()
	
	durationMs := float64(duration.Nanoseconds()) / 1000000.0
	
	// Simple P95 approximation - in production this would use a proper percentile calculation
	if durationMs > e.stats.ResponseTimeP95 {
		e.stats.ResponseTimeP95 = durationMs * 0.95 // Weighted update
	}
}