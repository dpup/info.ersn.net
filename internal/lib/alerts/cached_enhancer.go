package alerts

import (
	"context"
	"log"
	"time"
)

// CachedAlertEnhancer wraps the existing AlertEnhancer with content-based caching
// This replaces the complex background processing infrastructure
type CachedAlertEnhancer struct {
	enhancer AlertEnhancer
	cache    AlertCache
	hasher   *ContentHasher
}

// AlertCache provides simple caching interface for enhanced alerts
// Will be implemented by the main Cache with the new methods we added
type AlertCache interface {
	SetEnhancedAlert(contentHash string, enhanced interface{}, ttl time.Duration) error
	GetEnhancedAlert(contentHash string) (interface{}, bool, error)
	IsEnhancedAlertCached(contentHash string) bool
}

// NewCachedAlertEnhancer creates an enhancer with content-based caching
func NewCachedAlertEnhancer(enhancer AlertEnhancer, cache AlertCache) *CachedAlertEnhancer {
	return &CachedAlertEnhancer{
		enhancer: enhancer,
		cache:    cache,
		hasher:   NewContentHasher(),
	}
}

// EnhanceAlert enhances an alert with content-based deduplication
// First checks cache, then calls OpenAI if needed, then caches result
func (c *CachedAlertEnhancer) EnhanceAlert(ctx context.Context, raw RawAlert) (EnhancedAlert, error) {
	// Generate content hash for deduplication
	contentHash := c.hasher.HashRawAlert(raw)
	
	// Check cache first
	if cached, found, err := c.cache.GetEnhancedAlert(contentHash); err == nil && found {
		if enhanced, ok := cached.(EnhancedAlert); ok {
			log.Printf("Cache hit for alert content hash %s", contentHash[:8])
			return enhanced, nil
		}
	}
	
	log.Printf("Cache miss for alert content hash %s - calling OpenAI", contentHash[:8])
	
	// Cache miss - call OpenAI
	enhanced, err := c.enhancer.EnhanceAlert(ctx, raw)
	if err != nil {
		log.Printf("OpenAI enhancement failed for %s: %v", contentHash[:8], err)
		return enhanced, err
	}
	
	// Cache the result with 24 hour TTL 
	// This prevents duplicate OpenAI calls for the same incident content
	ttl := 24 * time.Hour
	if err := c.cache.SetEnhancedAlert(contentHash, enhanced, ttl); err != nil {
		log.Printf("Failed to cache enhanced alert: %v", err)
		// Don't fail the request if caching fails
	} else {
		log.Printf("Cached enhanced alert with hash %s for 24h", contentHash[:8])
	}
	
	return enhanced, nil
}

// HealthCheck delegates to underlying enhancer
func (c *CachedAlertEnhancer) HealthCheck(ctx context.Context) error {
	return c.enhancer.HealthCheck(ctx)
}

// IsAlertCached checks if alert would be served from cache without triggering enhancement
func (c *CachedAlertEnhancer) IsAlertCached(raw RawAlert) bool {
	contentHash := c.hasher.HashRawAlert(raw)
	return c.cache.IsEnhancedAlertCached(contentHash)
}