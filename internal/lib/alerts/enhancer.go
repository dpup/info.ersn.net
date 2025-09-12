package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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


	// Create user prompt with raw alert data as JSON
	rawAlertJSON, _ := json.Marshal(raw)
	userPrompt := fmt.Sprintf(`Parse this traffic incident report and return structured JSON:

Raw Alert: %s

Extract structured information following the schema.
Focus on making the details field human-readable by removing technical abbreviations and jargon.
If a style_url is provided, incorporate the relevant traffic flow context from the StyleUrl definitions into your description (e.g., mention one-way control, lane restrictions, etc.).
For the condensed summary, follow the examples provided - do NOT include location, keep it under 120 characters.`,
		string(rawAlertJSON))

	// Determine response format based on model capability
	var responseFormat *openai.ChatCompletionResponseFormat
	if a.model == "gpt-4o" || a.model == "gpt-4o-mini" {
		// Use JSON Schema for models that support it
		responseFormat = &openai.ChatCompletionResponseFormat{
			Type:       openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &AlertEnhancementSchema,
		}
	} else {
		// Fall back to JSON object for older models
		responseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	// Make OpenAI API call with structured output request
	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: SystemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		ResponseFormat: responseFormat,
		Temperature:    0.3, // Lower temperature for more consistent structured output
		MaxTokens:      1000,
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
	if structured.Location.Description == "" {
		structured.Location.Description = raw.Location // Fallback to original location string
	}
	// Ensure coordinates are populated from raw alert location if missing
	if structured.Location.Latitude == 0 && structured.Location.Longitude == 0 {
		// This shouldn't happen if AI follows instructions, but safety fallback
		structured.Location.Description = raw.Location
	}

	// Validate enum fields
	if !isValidImpact(structured.Impact) {
		structured.Impact = "unknown"
	}
	if !isValidDuration(structured.Duration) {
		structured.Duration = "unknown"
	}

	// Use AI-generated condensed summary (trust the AI to follow instructions)
	// Only fallback to a simple format if completely missing
	if structured.CondensedSummary == "" {
		structured.CondensedSummary = structured.Details // Simple fallback
		if len(structured.CondensedSummary) > 147 {
			structured.CondensedSummary = structured.CondensedSummary[:147] + "..."
		}
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
