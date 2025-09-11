package alerts

import (
	"context"
	"time"
)

// RawAlert represents unprocessed alert data from Caltrans
type RawAlert struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Location    string    `json:"location"`
	Timestamp   time.Time `json:"timestamp"`
}

// StructuredDescription represents AI-processed alert information in standardized format
type StructuredDescription struct {
	TimeReported     string            `json:"time_reported,omitempty"`
	Details          string            `json:"details"`
	Location         string            `json:"location"`
	LastUpdate       string            `json:"last_update,omitempty"`
	Impact           string            `json:"impact"`         // enum: none, light, moderate, severe
	Duration         string            `json:"duration"`       // enum: unknown, < 1 hour, several hours, ongoing
	AdditionalInfo   map[string]string `json:"additional_info,omitempty"`
	CondensedSummary string            `json:"condensed_summary,omitempty"`
}

// EnhancedAlert represents a fully processed alert with AI enhancement
type EnhancedAlert struct {
	ID                    string                `json:"id"`
	OriginalDescription   string                `json:"original_description"`
	StructuredDescription StructuredDescription `json:"structured_description"`
	CondensedSummary      string                `json:"condensed_summary"`
	ProcessedAt           time.Time             `json:"processed_at"`
}

// AlertEnhancer interface defines AI-powered alert description enhancement
type AlertEnhancer interface {
	// Enhance single alert with AI processing
	EnhanceAlert(ctx context.Context, raw RawAlert) (EnhancedAlert, error)

	// Generate condensed summary format
	GenerateCondensedSummary(ctx context.Context, enhanced StructuredDescription) (string, error)

	// Health check for AI service
	HealthCheck(ctx context.Context) error
}

// NewAlertEnhancer is implemented in enhancer.go