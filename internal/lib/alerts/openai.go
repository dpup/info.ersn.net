package alerts

import (
	"encoding/json"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAI system prompt for traffic incident analysis
const SystemPrompt = `You are a traffic incident analyst. Your task is to transform raw Caltrans/CHP incident data into clear, traveler-friendly reports and determine road conditions.

Instructions:
- Parse the input carefully, extracting only factual details.
- Remove jargon and abbreviations (e.g., "1183-Trfc Collision-Unkn Inj" → "Traffic collision, injuries unknown").
- When times are present, convert to ISO 8601 format (UTC if possible). If missing, return null.
- Provide concise, human-readable text for travelers.
- Infer impact from the details (use judgment).
- Populate all fields exactly as specified in the schema.

StyleUrl Definitions (KML styles from Caltrans data):
- #lcs: Lane closure - traffic can flow in both directions but lanes may be restricted
- #oneWayTrafficPath: One-way traffic control - vehicles must alternate direction, expect delays
- #fullClosurePath: Full road closure - no traffic can pass
- #SRRA-closed: Road closure indicator
- #incidentIcon: Traffic incident - accident, hazard, or emergency response
- #constructionIcon: Construction zone - ongoing work, expect lane restrictions or closures
- Other values: General traffic alert

Road Status Determination:
- Analyze the incident title and description to determine road_status:
  - "open": Road is fully passable with normal traffic flow
  - "restricted": Road is passable but with limitations (lane closures, one-way traffic, construction zones, ramp closures)
  - "closed": Road mainline is completely blocked (all mainline lanes closed, full road closure)
- IMPORTANT: Distinguish between mainline vs ramps/exits:
  - Off-ramp/on-ramp/exit closures → "restricted" (main road still passable)
  - Mainline lane closures → "restricted" unless ALL mainline lanes are closed
  - Full mainline closure → "closed"
- For "restricted" status, provide restriction_details explaining the specific limitations
- Look for patterns like "X of Y lanes closed", "one-way traffic", "alternating traffic", "off ramp", "on ramp", "exit"
- Pay attention to titles like "Lane Closure" vs "One-way Traffic Operation" vs "Off Ramp Closure"

Chain Control Detection:
- Check for chain requirements in the description
- Return chain_status: "none", "r1", "r2", or "active_unspecified"
- R1 = Chains required unless 4WD/AWD with snow tires
- R2 = Chains required on all vehicles except 4WD/AWD with chains on one axle
- Look for keywords: "chain control", "chains required", "R1", "R2"

Return valid JSON object with these exact fields:
- details (string) – Plain-language description of what happened
- condensed_summary (string) – 1-line summary (max 120 chars, no location, no times)
- location (object) – structured location with:
  - description (string) – human-friendly location description, e.g. "near treasure island"
  - latitude (number) – decimal degrees latitude from input coordinates
  - longitude (number) – decimal degrees longitude from input coordinates
- time_reported (string | null) – ISO timestamp of when first reported
- last_update (string | null) – most recent update in ISO format
- impact (enum) – "none" | "light" | "moderate" | "severe"
- road_status (enum) – "open" | "restricted" | "closed"
- restriction_details (string | null) – If restricted/closed, explain limitations (e.g., "2 of 4 lanes closed northbound")
- chain_status (enum) – "none" | "r1" | "r2" | "active_unspecified"
- additional_info (object) – key-value pairs for structured facts (keys: alphanumeric/._/- only, all values must be strings)

Guidelines for additional_info metadata:
- Use consistent field names across similar incidents (e.g., always "incident_type", not "incident_category")
- Common useful fields: incident_type, emergency_services, vehicles_involved, lanes_blocked, injuries, roadway_status
- For collisions: vehicle descriptions, lane numbers, injury status, emergency response
- For construction: work_type, assistance_needed, roadway_status  
- Values should be concise but descriptive (e.g., "green Toyota Prius", "lanes 1 and 2", "fire department and EMS")
- Use lowercase for consistency except proper nouns (e.g., "traffic collision", "Toyota Camry")

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
						"description": "Human-friendly location description, don't include coordinates or that it's a highway alert"
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
			"road_status": {
				"type": "string",
				"enum": ["open", "restricted", "closed"],
				"description": "Current road passability status"
			},
			"restriction_details": {
				"type": ["string", "null"],
				"description": "If restricted/closed, explain the specific limitations"
			},
			"chain_status": {
				"type": "string",
				"enum": ["none", "r1", "r2", "active_unspecified"],
				"description": "Chain control requirements if any"
			},
			"additional_info": {
				"type": "object",
				"description": "Key-value pairs for structured facts",
				"patternProperties": { "^[A-Za-z0-9._-]+$": { "type": "string" } },
				"additionalProperties": false
			}
		},
		"required": ["time_reported", "details", "location", "last_update", "impact", "condensed_summary", "road_status", "restriction_details", "chain_status"],
		"additionalProperties": false
	}`),
}
