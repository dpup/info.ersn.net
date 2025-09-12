package alerts

import (
	"encoding/json"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAI system prompt for traffic incident analysis
const SystemPrompt = `You are a traffic incident analyst. Your task is to transform raw Caltrans/CHP incident data into clear, traveler-friendly reports.

Instructions:
- Parse the input carefully, extracting only factual details.
- Remove jargon and abbreviations (e.g., "1183-Trfc Collision-Unkn Inj" → "Traffic collision, injuries unknown").
- When times are present, convert to ISO 8601 format (UTC if possible). If missing, return null.
- Provide concise, human-readable text for travelers.
- Infer impact and duration from the details (use judgment).
- Populate all fields exactly as specified in the schema.

StyleUrl Definitions (KML styles from Caltrans data):
- #lcs: Lane closure - traffic can flow in both directions but lanes may be restricted
- #oneWayTrafficPath: One-way traffic control - vehicles must alternate direction, expect delays
- #incidentIcon: Traffic incident - accident, hazard, or emergency response
- #constructionIcon: Construction zone - ongoing work, expect lane restrictions or closures
- Other values: General traffic alert

Return valid JSON object with these exact fields:
- time_reported (string | null) – ISO timestamp of when first reported
- details (string) – Plain-language description of what happened
- location (object) – structured location with:
  - description (string) – human-friendly location description
  - latitude (number) – decimal degrees latitude from input coordinates  
  - longitude (number) – decimal degrees longitude from input coordinates
- last_update (string | null) – most recent update in ISO format
- impact (enum) – "none" | "light" | "moderate" | "severe"
- duration (enum) – "unknown" | "< 1 hour" | "several hours" | "ongoing"
- additional_info (object) – key-value pairs for structured facts (keys: alphanumeric/._/- only, all values must be strings)

Guidelines for additional_info metadata:
• Use consistent field names across similar incidents (e.g., always "incident_type", not "incident_category")
• Common useful fields: incident_type, emergency_services, vehicles_involved, lanes_blocked, injuries, roadway_status
• For collisions: vehicle descriptions, lane numbers, injury status, emergency response
• For construction: work_type, assistance_needed, roadway_status  
• Values should be concise but descriptive (e.g., "green Toyota Prius", "lanes 1 and 2", "fire department and EMS")
• Use lowercase for consistency except proper nouns (e.g., "traffic collision", "Toyota Camry")
- condensed_summary (string) – 1-line summary (max 120 chars, no location, no times)

How to write condensed summaries:
- CRITICAL: Do NOT include ANY location details (no highway names, mile markers, cities)
- CRITICAL: Do NOT include times or dates
- Focus on WHAT happened, not WHERE it happened  
- Imagine someone telling a friend the 3 second version
- Include enough detail to help someone understand the scope and type of incident, but no more

Good examples:
- Overturned vehicle off road, not visible from highway, EMS/fire en route.
- Tire debris in one lane, traffic hazard.
- 3-vehicle crash (UPS truck, Toyota RAV4, VW sedan), injuries unknown, tow en route.

Bad examples (include location):
- Traffic collision on Route 4 eastbound
- Construction work on Highway 101  
- Accident near mile marker 31`

// AlertEnhancementSchema defines the JSON schema for structured alert output
var AlertEnhancementSchema = openai.ChatCompletionResponseFormatJSONSchema{
	Name:   "alert_enhancement",
	Strict: true,
	Schema: json.RawMessage(`{
		"type": "object",
		"properties": {
			"time_reported": {
				"type": ["string", "null"],
				"description": "ISO timestamp of when first reported, null if not available"
			},
			"details": {
				"type": "string", 
				"description": "Long form, plain-language description of what happened"
			},
			"condensed_summary": {
				"type": "string",
				"maxLength": 120,
				"description": "Very short summary of incident, no location, max 120 chars"
			},
			"location": {
				"type": "object",
				"properties": {
					"description": {
						"type": "string",
						"description": "Human-friendly location description, don't include coordinates"
					},
					"latitude": {
						"type": "number",
						"description": "Decimal degrees latitude from input coordinates"
					},
					"longitude": {
						"type": "number", 
						"description": "Decimal degrees longitude from input coordinates"
					}
				},
				"required": ["description", "latitude", "longitude"],
				"additionalProperties": false
			},
			"last_update": {
				"type": ["string", "null"],
				"description": "Most recent update in ISO format, null if not available"
			},
			"impact": {
				"type": "string",
				"enum": ["none", "light", "moderate", "severe"],
				"description": "Traffic impact severity level"
			},
			"duration": {
				"type": "string",
				"enum": ["unknown", "< 1 hour", "several hours", "ongoing"],
				"description": "Expected duration of incident"
			},
			"additional_info": {
				"type": "object",
				"description": "Key-value pairs for structured facts",
				"patternProperties": { "^[A-Za-z0-9._-]+$": { "type": "string" } },
				"additionalProperties": false
			}
		},
		"required": ["time_reported", "details", "location", "last_update", "impact", "duration", "condensed_summary"],
		"additionalProperties": false
	}`),
}
