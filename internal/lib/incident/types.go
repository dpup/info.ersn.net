// Package incident provides content-based incident processing and caching
// Implements background processing for OpenAI-enhanced Caltrans incident data
package incident

import (
	"context"
	"time"
)

// ProcessingStage represents different levels of incident data processing
type ProcessingStage int

const (
	// RAW_KML represents parsed KML data with no additional processing
	RAW_KML ProcessingStage = iota
	// ROUTE_FILTERED represents data after geographic filtering for specific routes
	ROUTE_FILTERED
	// OPENAI_ENHANCED represents data after OpenAI processing (most expensive)
	OPENAI_ENHANCED
)

func (p ProcessingStage) String() string {
	switch p {
	case RAW_KML:
		return "raw_kml"
	case ROUTE_FILTERED:
		return "route_filtered"
	case OPENAI_ENHANCED:
		return "openai_enhanced"
	default:
		return "unknown"
	}
}

// IncidentContentHash provides deterministic content-based identification
type IncidentContentHash struct {
	// ContentHash is SHA-256 of normalized incident description + location + category
	ContentHash       string    `json:"content_hash"`
	// NormalizedText is the cleaned version used for hashing (handles text variations)
	NormalizedText    string    `json:"normalized_text"`
	// LocationKey combines lat/lng with precision appropriate for incident matching
	LocationKey       string    `json:"location_key"`
	// IncidentCategory from Caltrans feed (closure, chain_control, chp_incident)
	IncidentCategory  string    `json:"incident_category"`
	// FirstSeenAt timestamp when hash was first generated
	FirstSeenAt       time.Time `json:"first_seen_at"`
}

// ProcessedIncident represents background-processed incident data
type ProcessedIncident struct {
	ContentHash        IncidentContentHash `json:"content_hash"`
	Stage             ProcessingStage     `json:"stage"`
	OriginalIncident  interface{}         `json:"original_incident"`
	ProcessedData     interface{}         `json:"processed_data"`
	LastSeenInFeed    time.Time          `json:"last_seen_in_feed"`
	CacheExpiresAt    time.Time          `json:"cache_expires_at"`
	ServeCount        int64              `json:"serve_count"`
	ProcessingDuration time.Duration     `json:"processing_duration"`
}

// IncidentContentHasher creates deterministic content hashes for incident deduplication
type IncidentContentHasher interface {
	// HashIncident creates a content hash for any incident type
	// Must be deterministic and handle minor text variations
	HashIncident(ctx context.Context, incident interface{}) (IncidentContentHash, error)
	
	// NormalizeIncidentText cleans text for consistent hashing
	// Handles case, whitespace, punctuation variations
	NormalizeIncidentText(text string) string
	
	// ValidateContentHash ensures hash meets integrity requirements
	ValidateContentHash(hash IncidentContentHash) error
}

// IncidentStore provides content-based storage with background processing
type IncidentStore interface {
	// GetProcessed retrieves cached processed data by content hash and stage
	// Returns nil if not found or expired
	GetProcessed(ctx context.Context, contentHash IncidentContentHash, stage ProcessingStage) (*ProcessedIncident, error)
	
	// StoreProcessed caches the result of background processing
	// Updates LastSeenInFeed if incident already exists
	StoreProcessed(ctx context.Context, entry ProcessedIncident) error
	
	// MarkSeenInCurrentFeed updates LastSeenInFeed to prevent premature expiration
	// Called during feed refresh to keep active incidents cached
	MarkSeenInCurrentFeed(ctx context.Context, contentHash IncidentContentHash) error
	
	// ExpireOldIncidents removes incidents not seen in feeds for configured duration
	// Implements "1 hour after incident disappears from feeds" requirement
	ExpireOldIncidents(ctx context.Context) (int, error)
	
	// GetCacheMetrics returns performance statistics for monitoring
	GetCacheMetrics(ctx context.Context) (ContentCacheMetrics, error)
}

// IncidentBatchProcessor handles out-of-band processing to achieve <200ms responses
type IncidentBatchProcessor interface {
	// StartBackgroundProcessing begins async processing of incidents
	// Processes incidents through OpenAI and caches results
	StartBackgroundProcessing(ctx context.Context) error
	
	// ProcessIncidentBatch handles a batch of incidents from feed refresh
	// Identifies new incidents and queues them for background processing
	ProcessIncidentBatch(ctx context.Context, incidents []interface{}) error
	
	// WarmCacheWithCurrentFeed processes current feed data to warm the cache
	WarmCacheWithCurrentFeed(ctx context.Context, incidents []interface{}) error
	
	// GetProcessingStats returns background processing performance metrics
	GetProcessingStats(ctx context.Context) (BatchProcessingStats, error)
	
	// Stop gracefully shuts down background processing
	Stop(ctx context.Context) error
}

// IncidentProcessor wraps OpenAI processing with background execution and caching
type IncidentProcessor interface {
	// GetEnhancedAlert returns enhanced alert immediately from cache if available
	// If not cached, returns raw alert and triggers background enhancement
	// Achieves <200ms response time by serving cached or raw data
	GetEnhancedAlert(ctx context.Context, incident interface{}) (interface{}, bool, error)
	
	// QueueForEnhancement adds incident to background processing queue
	// Used when cache miss occurs to ensure future requests are fast
	QueueForEnhancement(ctx context.Context, incident interface{}) error
	
	// GetEnhancementStatus returns cache hit rate and queue statistics
	GetEnhancementStatus(ctx context.Context) (EnhancementStatus, error)
	
	// StartEnhancementWorkers starts the background enhancement processing
	StartEnhancementWorkers(ctx context.Context) error
	
	// StopEnhancementWorkers gracefully stops background processing
	StopEnhancementWorkers(ctx context.Context) error
}

// Metrics and Statistics Types

// ContentCacheMetrics tracks cache effectiveness for content-based caching
type ContentCacheMetrics struct {
	// TotalCachedIncidents across all processing stages
	TotalCachedIncidents int64                       `json:"total_cached_incidents"`
	// CacheHitRate percentage (0.0-1.0) of requests served from cache
	CacheHitRate         float64                     `json:"cache_hit_rate"`
	// IncidentsByStage breakdown of cached incidents by processing level
	IncidentsByStage     map[ProcessingStage]int64   `json:"incidents_by_stage"`
	// AvgResponseTimeMs for different scenarios (cached vs uncached)
	AvgResponseTimeMs    map[string]float64          `json:"avg_response_time_ms"`
	// MemoryUsageBytes current cache memory consumption  
	MemoryUsageBytes     int64                       `json:"memory_usage_bytes"`
	// LastMetricsUpdate when these metrics were calculated
	LastMetricsUpdate    time.Time                   `json:"last_metrics_update"`
}

// BatchProcessingStats tracks out-of-band processing performance
type BatchProcessingStats struct {
	// QueuedIncidents waiting for background processing
	QueuedIncidents      int64         `json:"queued_incidents"`
	// ProcessedIncidents completed in current session
	ProcessedIncidents   int64         `json:"processed_incidents"`
	// FailedProcessing incidents that couldn't be enhanced
	FailedProcessing     int64         `json:"failed_processing"`
	// AvgProcessingTime for OpenAI enhancement
	AvgProcessingTime    time.Duration `json:"avg_processing_time"`
	// OpenAICallsSaved by content-based caching
	OpenAICallsSaved     int64         `json:"openai_calls_saved"`
	// CostSavingsEstimate based on avoided OpenAI API calls
	CostSavingsEstimate  float64       `json:"cost_savings_estimate"`
}

// EnhancementStatus provides real-time view of alert enhancement system
type EnhancementStatus struct {
	// CachedEnhancementsAvailable for immediate serving
	CachedEnhancementsAvailable int64   `json:"cached_enhancements_available"`
	// CacheHitRateLast24h for monitoring effectiveness
	CacheHitRateLast24h         float64 `json:"cache_hit_rate_last_24h"`
	// BackgroundQueueSize incidents waiting for processing
	BackgroundQueueSize         int64   `json:"background_queue_size"`
	// ResponseTimeP95 95th percentile API response time
	ResponseTimeP95             float64 `json:"response_time_p95_ms"`
	// IsHealthy indicates if system is meeting <200ms target
	IsHealthy                   bool    `json:"is_healthy"`
}