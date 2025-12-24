package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// WeatherAlertSystemPrompt is the OpenAI system prompt for weather alert enhancement
const WeatherAlertSystemPrompt = `You are a weather alert editor. Your task is to transform official National Weather Service (NWS) alert descriptions into clear, readable reports for travelers and residents.

## Your Goals

1. **Make it readable**: NWS alerts use a structured format with markers like "* WHAT...", "* WHERE...", etc. Convert this to natural, flowing prose.
2. **Focus on what matters**: Prioritize information that helps people make decisions - timing, severity, and practical impacts.
3. **Keep the facts**: Don't add information that isn't in the original. Don't speculate or exaggerate.
4. **Be concise**: Remove redundancy and bureaucratic language while preserving essential details.

## Output Format

Return a JSON object with exactly these fields:

- **headline**: A single sentence under 100 characters that captures the key threat and timing. This should be punchy and scannable.
- **summary**: 2-3 sentences of plain text. What's happening, when, and what should people do? Written for someone quickly checking conditions before travel.
- **details**: The full alert reformatted for readability. Use minimal markdown:
  - Use **bold** for category labels (e.g., "**Timing:**", "**Impacts:**")
  - Use line breaks to separate sections
  - Keep bullet points only if listing 3+ distinct items
  - Preserve specific numbers, measurements, and times from the original

## Guidelines by Alert Type

Adapt your approach based on the alert type:

- **Winter storms**: Emphasize snow amounts by elevation, wind speeds, travel impacts, chain control likelihood
- **Floods**: Focus on affected waterways, timing of peak levels, evacuation guidance
- **Wind**: Highlight gust speeds, duration, risks to vehicles and structures
- **Fire weather**: Note wind/humidity conditions, red flag details, evacuation status
- **Heat/Cold**: Temperature extremes, duration, vulnerable population guidance
- **Thunderstorms**: Timing, hail size, tornado potential, lightning risks

## Examples

### Example 1: Winter Storm Warning

**Input:**
* WHAT...Heavy snow expected. Accumulations up to a foot between
4500 to 5500 feet. 4 to 8 feet, with locally higher amounts above
5500 feet. Wind gusts up to 60 mph.

* WHERE...West Slope Northern Sierra Nevada and Western Plumas
County/Lassen Park Counties.

* WHEN...From 10 PM this evening to 10 PM PST Friday.

* IMPACTS...Dangerous travel conditions with chain controls and road
closures. Localized tree damage, power outages, and low visibility
due to the combination of heavy wet snow and strong winds.

* ADDITIONAL DETAILS...Snow accumulations will remain near or above
pass level through Tuesday, then drop to around 4500 to 5500 feet
during the day Wednesday through Friday. Snowfall rates of 1 to 2
inches per hour at times.

**Output:**
{
  "headline": "Heavy snow and 60 mph winds through Friday evening",
  "summary": "Expect dangerous travel conditions with 4-8 feet of snow above 5500 feet and wind gusts to 60 mph. Chain controls and road closures are likely. Avoid mountain travel if possible until conditions improve Friday evening.",
  "details": "**Snow amounts:**\n- Up to 1 foot between 4,500-5,500 feet\n- 4-8 feet above 5,500 feet, locally higher\n- Snowfall rates of 1-2 inches per hour at times\n\n**Wind:** Gusts up to 60 mph\n\n**Timing:** 10 PM tonight through 10 PM Friday. Snow levels will be at pass level through Tuesday, then drop to 4,500-5,500 feet Wednesday through Friday.\n\n**Impacts:** Dangerous travel conditions with chain controls and road closures likely. Expect tree damage, power outages, and low visibility from heavy wet snow and strong winds."
}

### Example 2: Flood Watch

**Input:**
* WHAT...Flooding caused by excessive rainfall continues to be
possible.

* WHERE...A portion of northern California, including the following
areas, the Sacramento Valley, northern San Joaquin Valley, Delta
region, Sierra Nevada and adjacent foothills, and Coastal Range.

* WHEN...Through Friday afternoon.

* IMPACTS...Excessive runoff will result in rises along area rivers,
creeks, streams. Small streams and creeks may rise out of their
banks. Flooding may occur in low-lying, poor drainage, and urban
areas. Mudslides and rockslides may occur in mountain and foothill
areas.

* ADDITIONAL DETAILS...
- Periods of moderate to heavy rain are forecast the week of
Christmas. Debris flows are not expected over recent burn
scars in northern California, but do anticipate enhanced
runoff in/below scars.
- http://www.weather.gov/safety/flood

**Output:**
{
  "headline": "FloodWatch through Friday from heavy rainfall",
  "summary": "Periods of moderate to heavy rain may cause flooding in low-lying areas, near streams, and in urban areas through Friday afternoon. Watch for rising water and avoid flooded roads. Mudslides possible in mountain areas.",
  "details": "**Timing:** Through Friday afternoon\n\n**Affected areas:** Sacramento Valley, northern San Joaquin Valley, Delta region, Sierra Nevada foothills, and Coastal Range.\n\n**Impacts:**\n- Rivers, creeks, and streams may rise out of banks\n- Flooding possible in low-lying, poor drainage, and urban areas\n- Mudslides and rockslides possible in mountain and foothill areas\n- Enhanced runoff expected in and below burn scars\n\n**Safety:** Never drive through flooded roads. Visit weather.gov/safety/flood for more information."
}

### Example 3: High Wind Warning

**Input:**
* WHAT...West winds 30 to 40 mph with gusts up to 65 mph expected.

* WHERE...Santa Cruz Mountains and East Bay Hills.

* WHEN...From 4 PM this afternoon to 10 AM PST Wednesday.

* IMPACTS...Damaging winds will blow down large objects such as
trees and power lines. Power outages are expected. Travel will
be difficult, especially for high profile vehicles.

* ADDITIONAL DETAILS...Winds will be strongest overnight into
Wednesday morning. Strongest gusts will be along ridge tops.

**Output:**
{
  "headline": "Damaging winds up to 65 mph through Wednesday morning",
  "summary": "Strong west winds with gusts to 65 mph will down trees and power lines, causing power outages. Travel will be hazardous, especially for high-profile vehicles. Secure loose outdoor objects.",
  "details": "**Wind:** West 30-40 mph, gusts to 65 mph. Strongest along ridge tops.\n\n**Timing:** 4 PM today through 10 AM Wednesday. Peak winds overnight into Wednesday morning.\n\n**Impacts:**\n- Trees and power lines likely to be blown down\n- Power outages expected\n- Difficult travel, especially for trucks, trailers, and RVs\n\n**Precautions:** Secure loose outdoor objects. Avoid travel during peak wind periods if possible."
}

Remember: Your output must be valid JSON with the three fields: headline, summary, and details.`

// WeatherAlertEnhancementSchema defines the JSON schema for weather alert enhancement output
var WeatherAlertEnhancementSchema = openai.ChatCompletionResponseFormatJSONSchema{
	Name:   "weather_alert_enhancement",
	Strict: true,
	Schema: json.RawMessage(`{
		"type": "object",
		"properties": {
			"headline": {
				"type": "string",
				"maxLength": 100,
				"description": "Single sentence summary under 100 characters"
			},
			"summary": {
				"type": "string",
				"description": "2-3 sentences of plain text for quick reading"
			},
			"details": {
				"type": "string",
				"description": "Full reformatted alert with minimal markdown"
			}
		},
		"required": ["headline", "summary", "details"],
		"additionalProperties": false
	}`),
}

// RawWeatherAlert represents unprocessed weather alert data from OpenWeatherMap
type RawWeatherAlert struct {
	ID          string   `json:"id"`
	Event       string   `json:"event"`       // e.g., "Winter Storm Warning", "Flood Watch"
	SenderName  string   `json:"sender_name"` // e.g., "NWS Sacramento CA"
	Description string   `json:"description"` // Raw NWS description text
	Tags        []string `json:"tags"`        // e.g., ["Snow/Ice", "Wind"]
	Start       int64    `json:"start"`       // Unix timestamp
	End         int64    `json:"end"`         // Unix timestamp
}

// EnhancedWeatherAlert represents a weather alert with AI-enhanced fields
type EnhancedWeatherAlert struct {
	ID       string `json:"id"`
	Headline string `json:"headline"`
	Summary  string `json:"summary"`
	Details  string `json:"details"`
}

// WeatherAlertEnhancer interface defines AI-powered weather alert enhancement
type WeatherAlertEnhancer interface {
	// EnhanceWeatherAlert enhances a raw weather alert with AI processing
	EnhanceWeatherAlert(ctx context.Context, raw RawWeatherAlert) (EnhancedWeatherAlert, error)
}

// weatherAlertEnhancer implements the WeatherAlertEnhancer interface using OpenAI
type weatherAlertEnhancer struct {
	client *openai.Client
	model  string
}

// NewWeatherAlertEnhancer creates a new WeatherAlertEnhancer implementation
func NewWeatherAlertEnhancer(apiKey, model string) WeatherAlertEnhancer {
	if apiKey == "" {
		return &weatherAlertEnhancer{client: nil, model: model}
	}

	client := openai.NewClient(apiKey)
	return &weatherAlertEnhancer{
		client: client,
		model:  model,
	}
}

// EnhanceWeatherAlert enhances a raw weather alert using OpenAI
func (w *weatherAlertEnhancer) EnhanceWeatherAlert(ctx context.Context, raw RawWeatherAlert) (EnhancedWeatherAlert, error) {
	if w.client == nil {
		return EnhancedWeatherAlert{}, errors.New("OpenAI client not initialized - invalid API key")
	}

	// Create user prompt with the alert data
	userPrompt := fmt.Sprintf(`Please enhance this weather alert:

**Alert Type:** %s
**Issued By:** %s
**Tags:** %v

**Original Description:**
%s

Transform this into a readable format with headline, summary, and details.`,
		raw.Event,
		raw.SenderName,
		raw.Tags,
		raw.Description)

	// Determine response format based on model capability
	var responseFormat *openai.ChatCompletionResponseFormat
	if w.model == "gpt-4o" || w.model == "gpt-4o-mini" {
		responseFormat = &openai.ChatCompletionResponseFormat{
			Type:       openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &WeatherAlertEnhancementSchema,
		}
	} else {
		responseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	// Make OpenAI API call
	resp, err := w.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: w.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: WeatherAlertSystemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		ResponseFormat: responseFormat,
		Temperature:    0.3,
		MaxTokens:      1000,
	})

	if err != nil {
		return EnhancedWeatherAlert{}, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return EnhancedWeatherAlert{}, errors.New("no response from OpenAI API")
	}

	// Parse the JSON response
	var result struct {
		Headline string `json:"headline"`
		Summary  string `json:"summary"`
		Details  string `json:"details"`
	}

	jsonResponse := resp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(jsonResponse), &result); err != nil {
		return EnhancedWeatherAlert{}, fmt.Errorf("failed to parse OpenAI JSON response: %w", err)
	}

	// Validate and provide fallbacks
	if result.Headline == "" {
		result.Headline = raw.Event
	}
	if result.Summary == "" {
		result.Summary = raw.Description
		if len(result.Summary) > 200 {
			result.Summary = result.Summary[:197] + "..."
		}
	}
	if result.Details == "" {
		result.Details = raw.Description
	}

	return EnhancedWeatherAlert{
		ID:       raw.ID,
		Headline: result.Headline,
		Summary:  result.Summary,
		Details:  result.Details,
	}, nil
}
