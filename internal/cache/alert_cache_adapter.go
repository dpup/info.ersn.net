package cache

import (
	"time"
)

// AlertCacheAdapter makes the main Cache implement the AlertCache interface
// This adapter allows the alerts package to use our cache without direct dependency
type AlertCacheAdapter struct {
	cache *Cache
}

// NewAlertCacheAdapter creates an adapter for alert caching
func NewAlertCacheAdapter(cache *Cache) *AlertCacheAdapter {
	return &AlertCacheAdapter{cache: cache}
}

// SetEnhancedAlert implements AlertCache interface
func (a *AlertCacheAdapter) SetEnhancedAlert(contentHash string, enhanced interface{}, ttl time.Duration) error {
	return a.cache.SetEnhancedAlert(contentHash, enhanced, ttl)
}

// GetEnhancedAlert implements AlertCache interface  
func (a *AlertCacheAdapter) GetEnhancedAlert(contentHash string) (interface{}, bool, error) {
	return a.cache.GetEnhancedAlert(contentHash)
}

// IsEnhancedAlertCached implements AlertCache interface
func (a *AlertCacheAdapter) IsEnhancedAlertCached(contentHash string) bool {
	return a.cache.IsEnhancedAlertCached(contentHash)
}