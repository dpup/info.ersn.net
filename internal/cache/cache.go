package cache

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Cache provides thread-safe in-memory caching with TTL
// Implementation per data-model.md Cache Entry lines 227-241
type Cache struct {
	entries map[string]*CacheEntry
	mutex   sync.RWMutex
}

// CacheEntry represents a cached item with metadata
// Structure per data-model.md lines 227-241
type CacheEntry struct {
	Key             string    `json:"key"`
	Data            []byte    `json:"data"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	RefreshInterval time.Duration `json:"refresh_interval"`
	Source          string    `json:"source"`
}

// NewCache creates a new in-memory cache
func NewCache() *Cache {
	return &Cache{
		entries: make(map[string]*CacheEntry),
	}
}

// Set stores data in cache with TTL based on refresh interval
func (c *Cache) Set(key string, data interface{}, refreshInterval time.Duration, source string) error {
	// Serialize data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data for cache: %w", err)
	}

	now := time.Now()
	entry := &CacheEntry{
		Key:             key,
		Data:            jsonData,
		CreatedAt:       now,
		ExpiresAt:       now.Add(refreshInterval),
		RefreshInterval: refreshInterval,
		Source:          source,
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.entries[key] = entry
	return nil
}

// Get retrieves data from cache if not stale
func (c *Cache) Get(key string, result interface{}) (bool, error) {
	c.mutex.RLock()
	entry, exists := c.entries[key]
	c.mutex.RUnlock()

	if !exists {
		return false, nil
	}

	// Check if entry is stale
	if c.IsStale(key) {
		return false, nil
	}

	// Deserialize data
	if err := json.Unmarshal(entry.Data, result); err != nil {
		return false, fmt.Errorf("failed to unmarshal cached data: %w", err)
	}

	return true, nil
}

// IsStale checks if cache entry is stale (past expiration)
func (c *Cache) IsStale(key string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return true
	}

	return time.Now().After(entry.ExpiresAt)
}

// IsVeryStale checks if cache entry is very stale (2x refresh interval)
// Used for stale data detection per research.md default 10 minutes = 2x refresh interval
func (c *Cache) IsVeryStale(key string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return true
	}

	veryStaleThreshold := entry.CreatedAt.Add(entry.RefreshInterval * 2)
	return time.Now().After(veryStaleThreshold)
}

// GetWithMetadata retrieves data and cache metadata
func (c *Cache) GetWithMetadata(key string, result interface{}) (*CacheEntry, bool, error) {
	c.mutex.RLock()
	entry, exists := c.entries[key]
	c.mutex.RUnlock()

	if !exists {
		return nil, false, nil
	}

	// Return metadata even if stale (caller decides how to handle)
	if result != nil {
		if err := json.Unmarshal(entry.Data, result); err != nil {
			return entry, exists, fmt.Errorf("failed to unmarshal cached data: %w", err)
		}
	}

	return entry, exists, nil
}

// Delete removes an entry from cache
func (c *Cache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	delete(c.entries, key)
}

// Clear removes all entries from cache
func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.entries = make(map[string]*CacheEntry)
}

// Keys returns all cache keys
func (c *Cache) Keys() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	keys := make([]string, 0, len(c.entries))
	for key := range c.entries {
		keys = append(keys, key)
	}
	return keys
}

// Stats returns cache statistics
func (c *Cache) Stats() CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	now := time.Now()
	stats := CacheStats{
		TotalEntries: len(c.entries),
	}

	for _, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			stats.StaleEntries++
		} else {
			stats.FreshEntries++
		}
		
		// Update oldest/newest
		if stats.OldestEntry.IsZero() || entry.CreatedAt.Before(stats.OldestEntry) {
			stats.OldestEntry = entry.CreatedAt
		}
		if entry.CreatedAt.After(stats.NewestEntry) {
			stats.NewestEntry = entry.CreatedAt
		}
	}

	return stats
}

// CleanupStale removes all stale entries from cache
func (c *Cache) CleanupStale() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	var removed int

	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
			removed++
		}
	}

	return removed
}

// StartPeriodicCleanup starts a goroutine that periodically cleans up stale entries
func (c *Cache) StartPeriodicCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			removed := c.CleanupStale()
			if removed > 0 {
				// Log cleanup activity (placeholder)
				_ = removed
			}
		}
	}()
}

// CacheStats provides cache usage statistics
type CacheStats struct {
	TotalEntries  int
	FreshEntries  int
	StaleEntries  int
	OldestEntry   time.Time
	NewestEntry   time.Time
}

// ProcessedIncidentEntry represents cached background-processed incident data
type ProcessedIncidentEntry struct {
	ContentHash       string        `json:"content_hash"`
	Stage            string        `json:"stage"` // raw_kml, route_filtered, openai_enhanced
	OriginalIncident interface{}   `json:"original_incident"`
	ProcessedData    interface{}   `json:"processed_data"`
	LastSeenInFeed   time.Time     `json:"last_seen_in_feed"`
	CacheExpiresAt   time.Time     `json:"cache_expires_at"`
	ServeCount       int64         `json:"serve_count"`
	ProcessingDuration time.Duration `json:"processing_duration"`
}

// SetProcessedIncident stores a processed incident with content-based key
func (c *Cache) SetProcessedIncident(contentHash, stage string, entry ProcessedIncidentEntry) error {
	key := fmt.Sprintf("processed_incident:%s:%s", contentHash, stage)
	
	// Set cache expiration based on entry's CacheExpiresAt
	ttl := time.Until(entry.CacheExpiresAt)
	if ttl <= 0 {
		// Already expired, don't cache
		return nil
	}
	
	return c.Set(key, entry, ttl, "processed_incident")
}

// GetProcessedIncident retrieves a processed incident by content hash and stage
func (c *Cache) GetProcessedIncident(contentHash, stage string) (*ProcessedIncidentEntry, bool, error) {
	key := fmt.Sprintf("processed_incident:%s:%s", contentHash, stage)
	
	var entry ProcessedIncidentEntry
	found, err := c.Get(key, &entry)
	if err != nil {
		return nil, false, err
	}
	
	if !found {
		return nil, false, nil
	}
	
	// Check if entry has expired based on its own expiration time
	if time.Now().After(entry.CacheExpiresAt) {
		c.Delete(key)
		return nil, false, nil
	}
	
	// Increment serve count
	entry.ServeCount++
	c.SetProcessedIncident(contentHash, stage, entry) // Update with new count
	
	return &entry, true, nil
}

// MarkIncidentSeenInFeed updates LastSeenInFeed for all stages of an incident
func (c *Cache) MarkIncidentSeenInFeed(contentHash string, seenAt time.Time) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Update all stages of this incident
	stages := []string{"raw_kml", "route_filtered", "openai_enhanced"}
	for _, stage := range stages {
		key := fmt.Sprintf("processed_incident:%s:%s", contentHash, stage)
		if entry, exists := c.entries[key]; exists {
			var processedEntry ProcessedIncidentEntry
			if err := json.Unmarshal(entry.Data, &processedEntry); err == nil {
				processedEntry.LastSeenInFeed = seenAt
				processedEntry.CacheExpiresAt = seenAt.Add(1 * time.Hour) // 1 hour after last seen
				
				// Update the cache entry
				jsonData, err := json.Marshal(processedEntry)
				if err == nil {
					entry.Data = jsonData
					entry.ExpiresAt = processedEntry.CacheExpiresAt
				}
			}
		}
	}
	
	return nil
}

// ExpireOldProcessedIncidents removes incidents not seen in feeds for over 1 hour
func (c *Cache) ExpireOldProcessedIncidents() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	now := time.Now()
	var removed int
	
	for key, entry := range c.entries {
		// Only check processed incident entries
		if entry.Source != "processed_incident" {
			continue
		}
		
		var processedEntry ProcessedIncidentEntry
		if err := json.Unmarshal(entry.Data, &processedEntry); err != nil {
			continue
		}
		
		// Remove if cache expiration time has passed
		if now.After(processedEntry.CacheExpiresAt) {
			delete(c.entries, key)
			removed++
		}
	}
	
	return removed
}