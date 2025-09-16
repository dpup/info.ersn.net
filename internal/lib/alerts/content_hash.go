package alerts

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

// ContentHasher provides simple content-based deduplication for alerts
type ContentHasher struct{}

// NewContentHasher creates a new content hasher
func NewContentHasher() *ContentHasher {
	return &ContentHasher{}
}

// HashRawAlert creates a content hash for deduplication
// Much simpler than the complex incident hashing system
func (h *ContentHasher) HashRawAlert(raw RawAlert) string {
	// Normalize the description text for consistent hashing
	normalizedDesc := h.normalizeText(raw.Description)
	normalizedTitle := h.normalizeText(raw.Title)

	// Create a content signature including title, description and location
	// This catches the same incident reported with minor text variations
	contentSignature := fmt.Sprintf("%s|%s|%s|%s",
		normalizedTitle,
		normalizedDesc,
		h.normalizeText(raw.Location),
		raw.StyleUrl, // Include StyleUrl as it indicates incident type
	)
	
	// Generate SHA-256 hash
	hash := sha256.Sum256([]byte(contentSignature))
	return fmt.Sprintf("%x", hash)
}

// normalizeText cleans text for consistent hashing
// Handles common variations in Caltrans incident descriptions
func (h *ContentHasher) normalizeText(text string) string {
	// Convert to lowercase
	normalized := strings.ToLower(text)
	
	// Remove extra whitespace
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	
	// Remove common punctuation that varies
	normalized = regexp.MustCompile(`[.,;:!?()-]`).ReplaceAllString(normalized, "")
	
	// Remove time-specific elements that change but content remains same
	// e.g., "at 14:32" vs "at 14:35" for same incident
	normalized = regexp.MustCompile(`at \d{1,2}:\d{2}`).ReplaceAllString(normalized, "")
	normalized = regexp.MustCompile(`\d{1,2}/\d{1,2}/\d{4}`).ReplaceAllString(normalized, "")
	
	// Normalize common abbreviations
	replacements := map[string]string{
		"hwy":     "highway",
		"nb":      "northbound", 
		"sb":      "southbound",
		"eb":      "eastbound",
		"wb":      "westbound",
		"incident": "inc",
		"closure": "closed",
	}
	
	for abbrev, full := range replacements {
		normalized = strings.ReplaceAll(normalized, abbrev, full)
	}
	
	return strings.TrimSpace(normalized)
}