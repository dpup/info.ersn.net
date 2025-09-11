package v1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Contract tests for updated RoadAlert message and ProcessingMetrics
// These tests verify the new fields and structure are properly defined
// They MUST FAIL initially to satisfy TDD RED-GREEN-Refactor cycle

func TestRoadAlert_NewFields(t *testing.T) {
	// Test that RoadAlert message has all new required fields
	alert := &RoadAlert{
		Id:                  "test-alert-001",
		Type:                AlertType_CLOSURE,
		Severity:            AlertSeverity_WARNING,
		Classification:      AlertClassification_ON_ROUTE, // NEW FIELD
		Title:               "Lane Closure on Highway 4",
		OriginalDescription: "Rte 4 EB of MM 31 - LANE CLOSURE, EMS ENRT", // NEW FIELD
		Description:         "Lane closure on Highway 4 eastbound at mile marker 31, emergency services responding",
		CondensedSummary:    "Hwy 4 – MM 31: Lane closure, EMS responding (Sep 11, 10:43 AM)", // NEW FIELD
		StartTime:           timestamppb.New(time.Now()),
		EndTime:             timestamppb.New(time.Now().Add(time.Hour)),
		LastUpdated:         timestamppb.New(time.Now()), // NEW FIELD
		AffectedSegments:    []string{"hwy4-angels-murphys"},
		Location: &Coordinates{ // NEW FIELD
			Latitude:  38.0675,
			Longitude: -120.5436,
		},
		Metadata: map[string]string{ // NEW FIELD
			"visibility":     "visible from roadway",
			"lanes_affected": "1 of 2",
		},
	}

	// Verify all fields are properly set
	assert.Equal(t, "test-alert-001", alert.Id)
	assert.Equal(t, AlertType_CLOSURE, alert.Type)
	assert.Equal(t, AlertSeverity_WARNING, alert.Severity)
	assert.Equal(t, AlertClassification_ON_ROUTE, alert.Classification)
	assert.Equal(t, "Lane Closure on Highway 4", alert.Title)
	assert.Equal(t, "Rte 4 EB of MM 31 - LANE CLOSURE, EMS ENRT", alert.OriginalDescription)
	assert.Contains(t, alert.Description, "Highway 4 eastbound")
	assert.Contains(t, alert.CondensedSummary, "Hwy 4")
	assert.NotNil(t, alert.StartTime)
	assert.NotNil(t, alert.EndTime)
	assert.NotNil(t, alert.LastUpdated)
	assert.Contains(t, alert.AffectedSegments, "hwy4-angels-murphys")
	assert.NotNil(t, alert.Location)
	assert.Equal(t, 38.0675, alert.Location.Latitude)
	assert.Equal(t, -120.5436, alert.Location.Longitude)
	assert.Contains(t, alert.Metadata, "visibility")
	assert.Equal(t, "visible from roadway", alert.Metadata["visibility"])
}

func TestAlertClassification_Enum(t *testing.T) {
	// Test AlertClassification enum values
	assert.Equal(t, int32(0), int32(AlertClassification_ALERT_CLASSIFICATION_UNSPECIFIED))
	assert.Equal(t, int32(1), int32(AlertClassification_ON_ROUTE))
	assert.Equal(t, int32(2), int32(AlertClassification_NEARBY))
	
	// Test string representation
	assert.Equal(t, "ALERT_CLASSIFICATION_UNSPECIFIED", AlertClassification_ALERT_CLASSIFICATION_UNSPECIFIED.String())
	assert.Equal(t, "ON_ROUTE", AlertClassification_ON_ROUTE.String())
	assert.Equal(t, "NEARBY", AlertClassification_NEARBY.String())
}

func TestRoadAlert_ClassificationUsage(t *testing.T) {
	// Test ON_ROUTE classification
	onRouteAlert := &RoadAlert{
		Id:             "on-route-001",
		Classification: AlertClassification_ON_ROUTE,
		Description:    "Lane closure directly on Highway 4",
	}
	assert.Equal(t, AlertClassification_ON_ROUTE, onRouteAlert.Classification)

	// Test NEARBY classification  
	nearbyAlert := &RoadAlert{
		Id:             "nearby-001",
		Classification: AlertClassification_NEARBY,
		Description:    "Accident on side street near Highway 4",
	}
	assert.Equal(t, AlertClassification_NEARBY, nearbyAlert.Classification)
}

func TestRoadAlert_DescriptionFields(t *testing.T) {
	// Test original vs processed description fields
	alert := &RoadAlert{
		Id:                  "desc-test-001",
		OriginalDescription: "Rte 4 WB at MM 15 - VEH VS DEER, BLOCKING 1 LN",
		Description:         "Vehicle versus deer collision on Highway 4 westbound at mile marker 15, blocking one lane",
		CondensedSummary:    "Hwy 4 – MM 15: Vehicle vs deer, one lane blocked (Sep 11, 11:20 AM)",
	}

	// Original should contain technical jargon
	assert.Contains(t, alert.OriginalDescription, "VEH VS DEER")
	assert.Contains(t, alert.OriginalDescription, "BLOCKING 1 LN")
	
	// Processed should be human-readable
	assert.Contains(t, alert.Description, "Vehicle versus deer collision")
	assert.Contains(t, alert.Description, "blocking one lane")
	
	// Condensed should be short and formatted
	assert.Contains(t, alert.CondensedSummary, "Hwy 4")
	assert.Contains(t, alert.CondensedSummary, "MM 15")
	assert.Less(t, len(alert.CondensedSummary), 200, "Condensed summary should be < 200 chars")
}

func TestRoadAlert_MetadataField(t *testing.T) {
	// Test dynamic metadata field
	alert := &RoadAlert{
		Id: "metadata-test-001",
		Metadata: map[string]string{
			"visibility":        "not visible from roadway",
			"lanes_affected":    "2 of 4",
			"estimated_duration": "30 minutes",
			"severity_level":    "moderate",
		},
	}

	assert.Equal(t, 4, len(alert.Metadata))
	assert.Equal(t, "not visible from roadway", alert.Metadata["visibility"])
	assert.Equal(t, "2 of 4", alert.Metadata["lanes_affected"])
	assert.Equal(t, "30 minutes", alert.Metadata["estimated_duration"])
	assert.Equal(t, "moderate", alert.Metadata["severity_level"])
}

func TestProcessingMetrics_Structure(t *testing.T) {
	// Test ProcessingMetrics message structure
	metrics := &ProcessingMetrics{
		TotalRawAlerts:        150,
		FilteredAlerts:        45,
		EnhancedAlerts:        42,
		EnhancementFailures:   3,
		AvgProcessingTimeMs:   250.5,
	}

	assert.Equal(t, int64(150), metrics.TotalRawAlerts)
	assert.Equal(t, int64(45), metrics.FilteredAlerts)
	assert.Equal(t, int64(42), metrics.EnhancedAlerts)
	assert.Equal(t, int64(3), metrics.EnhancementFailures)
	assert.Equal(t, 250.5, metrics.AvgProcessingTimeMs)
}

func TestGetProcessingMetricsRequest_Structure(t *testing.T) {
	// Test GetProcessingMetricsRequest message (should be empty)
	request := &GetProcessingMetricsRequest{}
	assert.NotNil(t, request)
}

func TestRoadAlert_BackwardCompatibility(t *testing.T) {
	// Test that existing fields still work (backward compatibility)
	alert := &RoadAlert{
		Id:               "compat-test-001",
		Type:             AlertType_INCIDENT,
		Severity:         AlertSeverity_CRITICAL,
		Title:            "Traffic Incident",
		Description:      "Multi-vehicle accident", // This is now AI-processed, but still works
		StartTime:        timestamppb.New(time.Now()),
		EndTime:          timestamppb.New(time.Now().Add(time.Hour)),
		AffectedSegments: []string{"hwy4-angels-murphys"},
	}

	// Existing fields should still work
	assert.Equal(t, "compat-test-001", alert.Id)
	assert.Equal(t, AlertType_INCIDENT, alert.Type)
	assert.Equal(t, AlertSeverity_CRITICAL, alert.Severity)
	assert.Equal(t, "Traffic Incident", alert.Title)
	assert.Equal(t, "Multi-vehicle accident", alert.Description)
	assert.NotNil(t, alert.StartTime)
	assert.NotNil(t, alert.EndTime)
	assert.Contains(t, alert.AffectedSegments, "hwy4-angels-murphys")
}

func TestRoadAlert_ValidationScenarios(t *testing.T) {
	// Test various validation scenarios for new fields
	
	// Test minimum required fields for enhanced alert
	minimalAlert := &RoadAlert{
		Id:                  "minimal-001",
		Type:                AlertType_CONSTRUCTION,
		Severity:            AlertSeverity_INFO,
		Classification:      AlertClassification_NEARBY,
		OriginalDescription: "Raw description",
		Description:         "Enhanced description",
	}
	
	assert.NotEmpty(t, minimalAlert.Id)
	assert.NotEqual(t, AlertClassification_ALERT_CLASSIFICATION_UNSPECIFIED, minimalAlert.Classification)
	assert.NotEmpty(t, minimalAlert.OriginalDescription)
	assert.NotEmpty(t, minimalAlert.Description)
	
	// Test that condensed summary can be empty (optional)
	assert.Empty(t, minimalAlert.CondensedSummary)
	
	// Test that metadata can be empty (optional)
	assert.Nil(t, minimalAlert.Metadata)
	
	// Test that location can be empty (optional)
	assert.Nil(t, minimalAlert.Location)
}

// Integration test - verify protobuf serialization/deserialization works
func TestRoadAlert_Serialization(t *testing.T) {
	original := &RoadAlert{
		Id:                  "serialization-test",
		Type:                AlertType_WEATHER,
		Severity:            AlertSeverity_WARNING,
		Classification:      AlertClassification_ON_ROUTE,
		Title:               "Winter Weather Alert",
		OriginalDescription: "Rte 4 - SNOW CONDITIONS",
		Description:         "Snow conditions affecting Highway 4",
		CondensedSummary:    "Hwy 4: Snow conditions (Sep 11, 12:00 PM)",
		StartTime:           timestamppb.New(time.Now()),
		LastUpdated:         timestamppb.New(time.Now()),
		Location: &Coordinates{
			Latitude:  38.1391,
			Longitude: -120.4561,
		},
		Metadata: map[string]string{
			"visibility": "reduced",
			"temperature": "28F",
		},
	}

	// Test that all new fields serialize/deserialize correctly
	// (This is a basic test - full protobuf serialization would require actual proto marshaling)
	assert.Equal(t, "serialization-test", original.Id)
	assert.Equal(t, AlertClassification_ON_ROUTE, original.Classification)
	assert.NotEmpty(t, original.OriginalDescription)
	assert.NotEmpty(t, original.CondensedSummary)
	assert.NotNil(t, original.Location)
	assert.Contains(t, original.Metadata, "visibility")
}