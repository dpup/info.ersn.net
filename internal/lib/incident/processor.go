package incident

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// incidentBatchProcessor implements IncidentBatchProcessor interface
type incidentBatchProcessor struct {
	store     IncidentStore
	hasher    IncidentContentHasher
	enhancer  IncidentProcessor
	
	// Background processing state
	queue        chan interface{}
	processing   bool
	processingMu sync.RWMutex
	
	// Metrics
	stats BatchProcessingStats
	statsMu sync.RWMutex
	
	// Configuration
	maxConcurrent int
	timeout       time.Duration
}

// NewBackgroundIncidentProcessor creates a new background processor
func NewBackgroundIncidentProcessor(store IncidentStore, hasher IncidentContentHasher, enhancer IncidentProcessor) IncidentBatchProcessor {
	return &incidentBatchProcessor{
		store:         store,
		hasher:        hasher,
		enhancer:      enhancer,
		queue:         make(chan interface{}, 5000), // Buffer for 5,000 incidents (route-filtered)
		maxConcurrent: 5,                           // Default concurrent workers
		timeout:       30 * time.Second,            // Default processing timeout
	}
}

// StartBackgroundProcessing begins async processing of incidents
func (p *incidentBatchProcessor) StartBackgroundProcessing(ctx context.Context) error {
	p.processingMu.Lock()
	defer p.processingMu.Unlock()
	
	if p.processing {
		return fmt.Errorf("background processing already running")
	}
	
	p.processing = true
	
	// Start worker goroutines
	for i := 0; i < p.maxConcurrent; i++ {
		go p.worker(ctx, i)
	}
	
	log.Printf("Started background incident processing with %d workers", p.maxConcurrent)
	return nil
}

// ProcessIncidentBatch handles a batch of incidents from feed refresh
func (p *incidentBatchProcessor) ProcessIncidentBatch(ctx context.Context, incidents []interface{}) error {
	p.statsMu.Lock()
	defer p.statsMu.Unlock()
	
	processed := int64(0)
	
	for _, incident := range incidents {
		// Generate content hash for incident
		hash, err := p.hasher.HashIncident(ctx, incident)
		if err != nil {
			log.Printf("Failed to hash incident: %v", err)
			continue
		}
		
		// Check if we already have enhanced version cached
		cached, err := p.store.GetProcessed(ctx, hash, OPENAI_ENHANCED)
		if err != nil {
			log.Printf("Failed to check cache for incident %s: %v", hash.ContentHash, err)
			continue
		}
		
		if cached != nil {
			// Already processed, just mark as seen in current feed
			if err := p.store.MarkSeenInCurrentFeed(ctx, hash); err != nil {
				log.Printf("Failed to mark incident as seen: %v", err)
			}
			continue
		}
		
		// Queue for background processing
		select {
		case p.queue <- incident:
			processed++
		default:
			// Queue is full, log warning but continue
			log.Printf("Background processing queue full, dropping incident")
		}
	}
	
	p.stats.QueuedIncidents += processed
	return nil
}

// WarmCacheWithCurrentFeed processes current feed data to warm the cache
func (p *incidentBatchProcessor) WarmCacheWithCurrentFeed(ctx context.Context, incidents []interface{}) error {
	if len(incidents) == 0 {
		log.Printf("Cache warming: no incidents provided")
		return nil
	}
	
	log.Printf("Starting cache warming with %d current feed incidents...", len(incidents))
	
	// Process incidents through the background pipeline
	processed := 0
	for _, incident := range incidents {
		select {
		case <-ctx.Done():
			log.Printf("Cache warming cancelled after processing %d incidents", processed)
			return ctx.Err()
		case p.queue <- incident:
			processed++
		default:
			// Queue is full - skip this incident
			log.Printf("Queue full during cache warming, skipped incident")
		}
	}
	
	log.Printf("Cache warming queued %d incidents for background processing", processed)
	
	// Update statistics
	p.statsMu.Lock()
	p.stats.QueuedIncidents += int64(processed)
	p.statsMu.Unlock()
	
	return nil
}

// GetProcessingStats returns background processing performance metrics
func (p *incidentBatchProcessor) GetProcessingStats(ctx context.Context) (BatchProcessingStats, error) {
	p.statsMu.RLock()
	defer p.statsMu.RUnlock()
	
	// Return copy of current stats
	return p.stats, nil
}

// Stop gracefully shuts down background processing
func (p *incidentBatchProcessor) Stop(ctx context.Context) error {
	p.processingMu.Lock()
	defer p.processingMu.Unlock()
	
	if !p.processing {
		return nil
	}
	
	p.processing = false
	close(p.queue)
	
	log.Printf("Stopped background incident processing")
	return nil
}

// worker processes incidents from the queue
func (p *incidentBatchProcessor) worker(ctx context.Context, workerID int) {
	log.Printf("Background processor worker %d started", workerID)
	
	for {
		select {
		case <-ctx.Done():
			log.Printf("Background processor worker %d stopping due to context cancellation", workerID)
			return
		case incident, ok := <-p.queue:
			if !ok {
				log.Printf("Background processor worker %d stopping due to queue closure", workerID)
				return
			}
			
			// Process the incident
			p.processIncident(ctx, incident, workerID)
		}
	}
}

// processIncident handles a single incident processing
func (p *incidentBatchProcessor) processIncident(ctx context.Context, incident interface{}, workerID int) {
	startTime := time.Now()
	
	// Generate content hash
	hash, err := p.hasher.HashIncident(ctx, incident)
	if err != nil {
		p.incrementFailedProcessing()
		log.Printf("Worker %d: Failed to hash incident: %v", workerID, err)
		return
	}
	
	// Create processing context with timeout
	processCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	
	// Store raw incident first
	rawEntry := ProcessedIncident{
		ContentHash:        hash,
		Stage:             RAW_KML,
		OriginalIncident:  incident,
		ProcessedData:     incident, // Raw data is the same as original
		LastSeenInFeed:    time.Now(),
		CacheExpiresAt:    time.Now().Add(24 * time.Hour), // 24 hour cache
		ServeCount:        0,
		ProcessingDuration: time.Since(startTime),
	}
	
	if err := p.store.StoreProcessed(processCtx, rawEntry); err != nil {
		log.Printf("Worker %d: Failed to store raw incident: %v", workerID, err)
	}
	
	// Queue for OpenAI enhancement via AsyncAlertEnhancer
	if err := p.enhancer.QueueForEnhancement(processCtx, incident); err != nil {
		p.incrementFailedProcessing()
		log.Printf("Worker %d: Failed to queue for enhancement: %v", workerID, err)
		return
	}
	
	// Update statistics
	processingDuration := time.Since(startTime)
	p.updateProcessingStats(processingDuration)
	
	log.Printf("Worker %d: Processed incident %s in %v", workerID, hash.ContentHash[:8], processingDuration)
}

// incrementFailedProcessing safely increments failed processing count
func (p *incidentBatchProcessor) incrementFailedProcessing() {
	p.statsMu.Lock()
	defer p.statsMu.Unlock()
	p.stats.FailedProcessing++
}

// updateProcessingStats safely updates processing statistics
func (p *incidentBatchProcessor) updateProcessingStats(duration time.Duration) {
	p.statsMu.Lock()
	defer p.statsMu.Unlock()
	
	p.stats.ProcessedIncidents++
	p.stats.QueuedIncidents-- // Decrement queued since we just processed one
	
	// Update average processing time
	if p.stats.ProcessedIncidents == 1 {
		p.stats.AvgProcessingTime = duration
	} else {
		// Running average calculation
		prevTotal := p.stats.AvgProcessingTime * time.Duration(p.stats.ProcessedIncidents-1)
		p.stats.AvgProcessingTime = (prevTotal + duration) / time.Duration(p.stats.ProcessedIncidents)
	}
}