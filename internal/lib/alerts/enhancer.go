package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// alertEnhancer implements the AlertEnhancer interface using OpenAI
type alertEnhancer struct {
	client *openai.Client
	model  string
}

// NewAlertEnhancer creates a new AlertEnhancer implementation
func NewAlertEnhancer(apiKey, model string) AlertEnhancer {
	if apiKey == "" {
		return &alertEnhancer{client: nil, model: model} // Will cause errors - for testing
	}
	
	client := openai.NewClient(apiKey)
	return &alertEnhancer{
		client: client,
		model:  model,
	}
}

// EnhanceAlert enhances a raw alert using OpenAI GPT with structured output
func (a *alertEnhancer) EnhanceAlert(ctx context.Context, raw RawAlert) (EnhancedAlert, error) {
	if a.client == nil {
		return EnhancedAlert{}, errors.New("OpenAI client not initialized - invalid API key")
	}

	// Create the system prompt for structured output
	systemPrompt := `You are a traffic incident analyst specializing in converting technical road incident reports into clear, public-friendly descriptions. Extract structured information and generate readable summaries for travelers.

Return valid JSON with these exact fields:
- time_reported: Parse any timestamps (ISO format or null)
- details: Core incident info without jargon
- location: Human-readable location
- last_update: Most recent update time (ISO or null)
- impact: traffic impact level ("none", "light", "moderate", "severe")
- duration: expected duration ("unknown", "< 1 hour", "several hours", "ongoing")
- additional_info: Object with key-value pairs for specific details
- condensed_summary: Single line format "Highway – Location: Description (time)" under 150 chars`

	// Create user prompt with raw alert data
	userPrompt := fmt.Sprintf(`Parse this traffic incident report and return structured JSON:

Original: %s
Location Context: %s

Extract structured information following the schema. Focus on making the details field human-readable by removing technical abbreviations and jargon. Generate a condensed summary in the format: "Highway – Location: Brief description (time)".`,
		raw.Description,
		raw.Location)

	// Make OpenAI API call with structured output request
	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Temperature: 0.3, // Lower temperature for more consistent structured output
		MaxTokens:   1000,
	})

	if err != nil {
		return EnhancedAlert{}, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return EnhancedAlert{}, errors.New("no response from OpenAI API")
	}

	// Parse the JSON response
	var structured StructuredDescription
	jsonResponse := resp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(jsonResponse), &structured); err != nil {
		return EnhancedAlert{}, fmt.Errorf("failed to parse OpenAI JSON response: %w", err)
	}

	// Validate required fields
	if structured.Details == "" {
		structured.Details = raw.Description // Fallback to original
	}
	if structured.Location == "" {
		structured.Location = raw.Location // Fallback to original
	}
	
	// Validate enum fields
	if !isValidImpact(structured.Impact) {
		structured.Impact = "unknown"
	}
	if !isValidDuration(structured.Duration) {
		structured.Duration = "unknown"
	}

	// Generate condensed summary if not provided or invalid
	if structured.CondensedSummary == "" || len(structured.CondensedSummary) > 150 {
		summary, _ := a.GenerateCondensedSummary(ctx, structured)
		structured.CondensedSummary = summary
	}

	// Create enhanced alert
	enhanced := EnhancedAlert{
		ID:                    raw.ID,
		OriginalDescription:   raw.Description,
		StructuredDescription: structured,
		CondensedSummary:      structured.CondensedSummary,
		ProcessedAt:           time.Now(),
	}

	return enhanced, nil
}

// GenerateCondensedSummary creates a short, formatted summary
func (a *alertEnhancer) GenerateCondensedSummary(ctx context.Context, enhanced StructuredDescription) (string, error) {
	// Extract highway/route info from location
	highway := extractHighway(enhanced.Location)
	
	// Create basic summary format
	location := enhanced.Location
	if len(location) > 30 {
		// Truncate long locations
		location = location[:27] + "..."
	}
	
	details := enhanced.Details
	if len(details) > 50 {
		// Truncate long details
		details = details[:47] + "..."
	}
	
	// Generate timestamp
	timeStr := ""
	if enhanced.TimeReported != "" {
		if t, err := time.Parse(time.RFC3339, enhanced.TimeReported); err == nil {
			timeStr = t.Format("Jan 2, 3:04 PM")
		}
	}
	if timeStr == "" {
		timeStr = time.Now().Format("Jan 2, 3:04 PM")
	}
	
	// Format: "Highway – Location: Description (time)"
	summary := fmt.Sprintf("%s – %s: %s (%s)", highway, location, details, timeStr)
	
	// Ensure under 150 characters
	if len(summary) > 150 {
		// Progressively truncate to fit
		if len(details) > 20 {
			details = details[:17] + "..."
			summary = fmt.Sprintf("%s – %s: %s (%s)", highway, location, details, timeStr)
		}
		if len(summary) > 150 && len(location) > 15 {
			location = location[:12] + "..."
			summary = fmt.Sprintf("%s – %s: %s (%s)", highway, location, details, timeStr)
		}
		if len(summary) > 150 {
			// Last resort: truncate entire summary
			summary = summary[:147] + "..."
		}
	}
	
	return summary, nil
}

// HealthCheck verifies OpenAI API connectivity and rate limits
func (a *alertEnhancer) HealthCheck(ctx context.Context) error {
	if a.client == nil {
		return errors.New("OpenAI client not initialized")
	}

	// Make a minimal API call to test connectivity
	_, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Test",
			},
		},
		MaxTokens: 1,
	})

	if err != nil {
		return fmt.Errorf("OpenAI API health check failed: %w", err)
	}

	return nil
}

// Helper functions

// extractHighway extracts highway/route information from location string
func extractHighway(location string) string {
	location = strings.ToUpper(location)
	
	// Common highway patterns
	if strings.Contains(location, "HWY 4") || strings.Contains(location, "HIGHWAY 4") {
		return "Hwy 4"
	}
	if strings.Contains(location, "HWY 49") || strings.Contains(location, "HIGHWAY 49") {
		return "Hwy 49"
	}
	if strings.Contains(location, "US 50") || strings.Contains(location, "HIGHWAY 50") {
		return "US 50"
	}
	if strings.Contains(location, "SR-65") || strings.Contains(location, "STATE ROUTE 65") {
		return "SR-65"
	}
	
	// Generic route extraction
	if strings.Contains(location, "ROUTE ") {
		return "Route"
	}
	if strings.Contains(location, "HIGHWAY") {
		return "Highway"
	}
	
	return "Road"
}

// isValidImpact validates impact enum values
func isValidImpact(impact string) bool {
	validImpacts := []string{"none", "light", "moderate", "severe"}
	for _, valid := range validImpacts {
		if impact == valid {
			return true
		}
	}
	return false
}

// isValidDuration validates duration enum values
func isValidDuration(duration string) bool {
	validDurations := []string{"unknown", "< 1 hour", "several hours", "ongoing"}
	for _, valid := range validDurations {
		if duration == valid {
			return true
		}
	}
	return false
}