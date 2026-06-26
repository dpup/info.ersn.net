package hazards

import (
	"strings"

	api "github.com/dpup/info.ersn.net/server/api/v1"
)

// The unified severity scale (docs/hazard-aggregation-design.md §4.2). It
// expresses response urgency to the public, not physical magnitude — an
// editorial prioritization shared across all sources so a client can sort
// "most urgent first" and color a map without source-specific logic.
const (
	SevInfo     = "INFO"
	SevMinor    = "MINOR"
	SevModerate = "MODERATE"
	SevSevere   = "SEVERE"
	SevExtreme  = "EXTREME"
)

// severityRank maps a unified severity to its 0..4 rank (for sort/color).
func severityRank(s string) int {
	switch s {
	case SevExtreme:
		return 4
	case SevSevere:
		return 3
	case SevModerate:
		return 2
	case SevMinor:
		return 1
	default:
		return 0
	}
}

// fromAlertSeverity maps the shared api.AlertSeverity enum onto the unified
// scale (road incidents reuse it). Every enum value maps, incl. UNSPECIFIED.
func fromAlertSeverity(s api.AlertSeverity) string {
	switch s {
	case api.AlertSeverity_CRITICAL:
		return SevSevere
	case api.AlertSeverity_WARNING:
		return SevModerate
	case api.AlertSeverity_INFO:
		return SevMinor
	default: // ALERT_SEVERITY_UNSPECIFIED
		return SevInfo
	}
}

// fromChainLevel maps a chain-control level onto the unified scale.
func fromChainLevel(l api.ChainControlLevel) string {
	switch l {
	case api.ChainControlLevel_CHAIN_CONTROL_LEVEL_R3:
		return SevSevere
	case api.ChainControlLevel_CHAIN_CONTROL_LEVEL_R2:
		return SevModerate
	case api.ChainControlLevel_CHAIN_CONTROL_LEVEL_R1:
		return SevMinor
	default:
		return SevInfo
	}
}

// fromNWSSeverity maps an NWS severity string onto the unified scale.
func fromNWSSeverity(s string) string {
	switch s {
	case "Extreme":
		return SevExtreme
	case "Severe":
		return SevSevere
	case "Moderate":
		return SevModerate
	case "Minor":
		return SevMinor
	default:
		return SevInfo
	}
}

// fromFireWeatherState maps a fire-weather state string ("normal"|"elevated"|
// "red-flag" or their UPPER enum names) onto the unified scale.
func fromFireWeatherState(state string) string {
	switch strings.ToLower(state) {
	case "red-flag", "red_flag":
		return SevSevere
	case "elevated":
		return SevModerate
	default:
		return SevInfo
	}
}

// fromMagnitude maps an earthquake magnitude onto the unified scale.
func fromMagnitude(m float64) string {
	switch {
	case m >= 5:
		return SevSevere
	case m >= 4:
		return SevModerate
	case m >= 2.5:
		return SevMinor
	default:
		return SevInfo
	}
}

// fromChainLevelStr maps a Caltrans chain-control level string ("R1"|"R2"|"R3")
// onto the unified scale.
func fromChainLevelStr(level string) string {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "R3":
		return SevSevere
	case "R2":
		return SevModerate
	case "R1":
		return SevMinor
	default:
		return SevInfo
	}
}
