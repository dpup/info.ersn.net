package contract

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"github.com/dpup/info.ersn.net/server/internal/lib/incident"
)

// IncidentContentHasher interface contract test
// This test MUST fail until the IncidentContentHasher interface is implemented
// TDD REQUIREMENT: These tests must fail before any implementation exists

// MockIncident represents a test incident for hashing
type MockIncident struct {
	Description string
	Latitude    float64
	Longitude   float64
	Category    string
	URL         string
}

// Use the actual types from the incident package
type IncidentContentHasher = incident.IncidentContentHasher
type IncidentContentHash = incident.IncidentContentHash

// TestIncidentContentHasher_HashIncident tests deterministic hash generation
func TestIncidentContentHasher_HashIncident(t *testing.T) {
	// Implementation is now available - run the test!
	hasher := incident.NewIncidentContentHasher()
	ctx := context.Background()
	
	testIncident := MockIncident{
		Description: "I-80 WESTBOUND CHAIN CONTROLS REQUIRED FROM DRUM TO NYACK",
		Latitude:    39.1234,
		Longitude:   -120.5678,
		Category:    "chain_control",
		URL:         "chain_controls.kml",
	}
	
	// Test deterministic hashing
	hash1, err := hasher.HashIncident(ctx, testIncident)
	require.NoError(t, err, "First hash generation should not error")
	
	hash2, err := hasher.HashIncident(ctx, testIncident)
	require.NoError(t, err, "Second hash generation should not error")
	
	// Same incident should produce identical hashes
	assert.Equal(t, hash1.ContentHash, hash2.ContentHash, "Same incident should produce same content hash")
	assert.Equal(t, hash1.LocationKey, hash2.LocationKey, "Same incident should produce same location key")
	assert.Equal(t, hash1.IncidentCategory, hash2.IncidentCategory, "Same incident should have same category")
	
	// Hash should be SHA-256 length (64 hex characters)
	assert.Len(t, hash1.ContentHash, 64, "Content hash should be 64 characters (SHA-256)")
	assert.Regexp(t, "^[a-f0-9]{64}$", hash1.ContentHash, "Content hash should be lowercase hex")
	
	// Normalized text should be cleaned
	assert.NotEmpty(t, hash1.NormalizedText, "Normalized text should not be empty")
	assert.NotEqual(t, testIncident.Description, hash1.NormalizedText, "Normalized text should differ from original")
}

// TestIncidentContentHasher_NormalizeIncidentText tests text normalization
func TestIncidentContentHasher_NormalizeIncidentText(t *testing.T) {
	// Implementation is now available - run the test!
	hasher := incident.NewIncidentContentHasher()
	
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase and trim whitespace",
			input:    "  I-80 WESTBOUND CHAIN CONTROLS  ",
			expected: "i-80 westbound chain controls",
		},
		{
			name:     "remove extra punctuation",
			input:    "I-80 CHAIN CONTROLS!!! REQUIRED.",
			expected: "i-80 chain controls required",
		},
		{
			name:     "normalize multiple spaces",
			input:    "I-80    CHAIN   CONTROLS   REQUIRED",
			expected: "i-80 chain controls required",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasher.NormalizeIncidentText(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestIncidentContentHasher_ValidateContentHash tests hash validation
func TestIncidentContentHasher_ValidateContentHash(t *testing.T) {
	// Implementation is now available - run the test!
	hasher := incident.NewIncidentContentHasher()
	
	validHash := IncidentContentHash{
		ContentHash:      "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678",
		NormalizedText:   "i-80 chain controls required",
		LocationKey:      "39.123_-120.567",
		IncidentCategory: "chain_control",
		FirstSeenAt:      time.Now(),
	}
	
	// Valid hash should pass
	err := hasher.ValidateContentHash(validHash)
	assert.NoError(t, err, "Valid hash should pass validation")
	
	// Invalid hash should fail
	invalidHash := validHash
	invalidHash.ContentHash = "invalid"
	err = hasher.ValidateContentHash(invalidHash)
	assert.Error(t, err, "Invalid content hash should fail validation")
	
	// Empty category should fail
	invalidHash = validHash
	invalidHash.IncidentCategory = ""
	err = hasher.ValidateContentHash(invalidHash)
	assert.Error(t, err, "Empty category should fail validation")
}

// TestIncidentContentHasher_DifferentIncidents tests different incidents produce different hashes
func TestIncidentContentHasher_DifferentIncidents(t *testing.T) {
	// Implementation is now available - run the test!
	hasher := incident.NewIncidentContentHasher()
	ctx := context.Background()
	
	incident1 := MockIncident{
		Description: "I-80 WESTBOUND CHAIN CONTROLS",
		Latitude:    39.1234,
		Longitude:   -120.5678,
		Category:    "chain_control",
	}
	
	incident2 := MockIncident{
		Description: "I-80 EASTBOUND LANE CLOSURE",
		Latitude:    39.1234,
		Longitude:   -120.5678,
		Category:    "closure",
	}
	
	hash1, err := hasher.HashIncident(ctx, incident1)
	require.NoError(t, err)
	
	hash2, err := hasher.HashIncident(ctx, incident2)
	require.NoError(t, err)
	
	// Different incidents should produce different hashes
	assert.NotEqual(t, hash1.ContentHash, hash2.ContentHash, "Different incidents should have different content hashes")
	assert.NotEqual(t, hash1.IncidentCategory, hash2.IncidentCategory, "Different categories should be preserved")
}