package incident

import (
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// incidentContentHasher implements IncidentContentHasher interface
type incidentContentHasher struct{}

// NewIncidentContentHasher creates a new incident content hasher
func NewIncidentContentHasher() IncidentContentHasher {
	return &incidentContentHasher{}
}

// HashIncident creates a content hash for any incident type
func (h *incidentContentHasher) HashIncident(ctx context.Context, incident interface{}) (IncidentContentHash, error) {
	// Extract incident data using reflection to handle various input types
	incidentData, err := h.extractIncidentData(incident)
	if err != nil {
		return IncidentContentHash{}, fmt.Errorf("failed to extract incident data: %w", err)
	}
	
	// Normalize the description text
	normalizedText := h.NormalizeIncidentText(incidentData.Description)
	
	// Generate location key with appropriate precision
	locationKey := h.generateLocationKey(incidentData.Latitude, incidentData.Longitude)
	
	// Create content string for hashing: normalized_text + location + category
	contentString := fmt.Sprintf("%s|%s|%s", normalizedText, locationKey, incidentData.Category)
	
	// Generate SHA-256 hash
	hash := sha256.Sum256([]byte(contentString))
	contentHash := fmt.Sprintf("%x", hash)
	
	return IncidentContentHash{
		ContentHash:      contentHash,
		NormalizedText:   normalizedText,
		LocationKey:      locationKey,
		IncidentCategory: incidentData.Category,
		FirstSeenAt:      time.Now(),
	}, nil
}

// NormalizeIncidentText cleans text for consistent hashing
func (h *incidentContentHasher) NormalizeIncidentText(text string) string {
	// Convert to lowercase
	normalized := strings.ToLower(text)
	
	// Trim whitespace
	normalized = strings.TrimSpace(normalized)
	
	// Replace multiple spaces with single space
	spaceRegex := regexp.MustCompile(`\s+`)
	normalized = spaceRegex.ReplaceAllString(normalized, " ")
	
	// Remove common punctuation that doesn't affect meaning
	punctRegex := regexp.MustCompile(`[.!?:;,]+$`)
	normalized = punctRegex.ReplaceAllString(normalized, "")
	
	// Remove extra punctuation within text (keep hyphens and parentheses)
	extraPunctRegex := regexp.MustCompile(`[.!?:;,]{2,}`)
	normalized = extraPunctRegex.ReplaceAllString(normalized, "")
	
	return normalized
}

// ValidateContentHash ensures hash meets integrity requirements
func (h *incidentContentHasher) ValidateContentHash(hash IncidentContentHash) error {
	// Validate content hash format (64 character hex string)
	if len(hash.ContentHash) != 64 {
		return fmt.Errorf("content hash must be 64 characters, got %d", len(hash.ContentHash))
	}
	
	hexRegex := regexp.MustCompile(`^[a-f0-9]{64}$`)
	if !hexRegex.MatchString(hash.ContentHash) {
		return fmt.Errorf("content hash must be lowercase hex, got: %s", hash.ContentHash)
	}
	
	// Validate required fields
	if hash.NormalizedText == "" {
		return fmt.Errorf("normalized text cannot be empty")
	}
	
	if hash.LocationKey == "" {
		return fmt.Errorf("location key cannot be empty")
	}
	
	if hash.IncidentCategory == "" {
		return fmt.Errorf("incident category cannot be empty")
	}
	
	// Validate category is one of expected types
	validCategories := map[string]bool{
		"chain_control": true,
		"closure":       true,
		"chp_incident":  true,
		"construction":  true,
		"traffic":       true,
		"test":          true, // For testing
	}
	
	if !validCategories[hash.IncidentCategory] {
		return fmt.Errorf("invalid incident category: %s", hash.IncidentCategory)
	}
	
	// Validate FirstSeenAt is not in the future
	if hash.FirstSeenAt.After(time.Now().Add(1 * time.Minute)) { // Allow 1 minute tolerance
		return fmt.Errorf("first seen time cannot be in the future")
	}
	
	return nil
}

// extractIncidentData extracts data from various incident input types
func (h *incidentContentHasher) extractIncidentData(incident interface{}) (*incidentData, error) {
	// Handle different input types using reflection
	v := reflect.ValueOf(incident)
	
	// Dereference pointers
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, fmt.Errorf("incident cannot be nil")
		}
		v = v.Elem()
	}
	
	switch v.Kind() {
	case reflect.Map:
		return h.extractFromMap(v.Interface())
	case reflect.Struct:
		return h.extractFromStruct(v.Interface())
	default:
		return nil, fmt.Errorf("unsupported incident type: %T", incident)
	}
}

// extractFromMap extracts incident data from map[string]interface{}
func (h *incidentContentHasher) extractFromMap(incident interface{}) (*incidentData, error) {
	incidentMap, ok := incident.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected map[string]interface{}, got %T", incident)
	}
	
	data := &incidentData{}
	
	// Extract description
	if desc, exists := incidentMap["description"]; exists {
		if descStr, ok := desc.(string); ok {
			data.Description = descStr
		}
	}
	
	// Extract latitude
	if lat, exists := incidentMap["latitude"]; exists {
		data.Latitude = h.extractFloat64(lat)
	}
	
	// Extract longitude
	if lng, exists := incidentMap["longitude"]; exists {
		data.Longitude = h.extractFloat64(lng)
	}
	
	// Extract category
	if cat, exists := incidentMap["category"]; exists {
		if catStr, ok := cat.(string); ok {
			data.Category = catStr
		}
	}
	
	// Validate required fields
	if data.Description == "" {
		return nil, fmt.Errorf("incident description is required")
	}
	
	if data.Category == "" {
		return nil, fmt.Errorf("incident category is required")
	}
	
	return data, nil
}

// extractFromStruct extracts incident data from struct types
func (h *incidentContentHasher) extractFromStruct(incident interface{}) (*incidentData, error) {
	v := reflect.ValueOf(incident)
	t := reflect.TypeOf(incident)
	
	data := &incidentData{}
	
	// Iterate through struct fields
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		fieldName := strings.ToLower(fieldType.Name)
		
		if !field.CanInterface() {
			continue
		}
		
		switch fieldName {
		case "description", "descriptiontext":
			if field.Kind() == reflect.String {
				data.Description = field.String()
			}
		case "latitude", "lat":
			data.Latitude = h.extractFloat64(field.Interface())
		case "longitude", "lng", "lon":
			data.Longitude = h.extractFloat64(field.Interface())
		case "category":
			if field.Kind() == reflect.String {
				data.Category = field.String()
			}
		case "feedtype":
			// Handle CaltransIncident FeedType field
			if field.Kind() == reflect.Int {
				feedType := int(field.Int())
				// Map feed type to category string
				switch feedType {
				case 0: // CHAIN_CONTROL
					data.Category = "chain_control"
				case 1: // LANE_CLOSURE
					data.Category = "lane_closure"
				case 2: // CHP_INCIDENT
					data.Category = "chp_incident"
				default:
					data.Category = "unknown"
				}
			}
		case "coordinates":
			// Handle CaltransIncident Coordinates field (api.Coordinates struct)
			if field.Kind() == reflect.Ptr && !field.IsNil() {
				coords := field.Interface()
				// Extract lat/lng from api.Coordinates struct
				coordsValue := reflect.ValueOf(coords).Elem()
				if coordsValue.IsValid() {
					if latField := coordsValue.FieldByName("Latitude"); latField.IsValid() {
						data.Latitude = h.extractFloat64(latField.Interface())
					}
					if lngField := coordsValue.FieldByName("Longitude"); lngField.IsValid() {
						data.Longitude = h.extractFloat64(lngField.Interface())
					}
				}
			}
		}
	}
	
	// Validate required fields
	if data.Description == "" {
		return nil, fmt.Errorf("incident description is required")
	}
	
	if data.Category == "" {
		return nil, fmt.Errorf("incident category is required")
	}
	
	return data, nil
}

// extractFloat64 safely extracts float64 from various numeric types
func (h *incidentContentHasher) extractFloat64(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0.0
}

// generateLocationKey creates a location key with appropriate precision for incident matching
func (h *incidentContentHasher) generateLocationKey(lat, lng float64) string {
	// Use 3 decimal places for precision (~100m accuracy)
	// This balances precision with duplicate detection
	return fmt.Sprintf("%.3f_%.3f", lat, lng)
}

// incidentData holds extracted incident information
type incidentData struct {
	Description string
	Latitude    float64
	Longitude   float64
	Category    string
}