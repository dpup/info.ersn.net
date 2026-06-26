package services

import (
	"testing"
	"time"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/clients/nws"
)

func TestNWSAlertsToProto_NoDuplicateText(t *testing.T) {
	in := []nws.Alert{{
		ID:          "urn:oid:abc",
		Event:       "Red Flag Warning",
		Severity:    "Severe",
		Headline:    "Red Flag Warning in effect until 8 PM",
		Description: "Gusty winds and low humidity will create critical fire conditions.",
		SenderName:  "NWS Sacramento CA",
		Effective:   time.Unix(1782400000, 0),
		Expires:     time.Unix(1782490000, 0),
		Zones:       []string{"CAZ064"},
	}}

	out := nwsAlertsToProto(in)
	if len(out) != 1 {
		t.Fatalf("got %d alerts, want 1", len(out))
	}
	a := out[0]

	if a.Source != api.AlertSource_NWS {
		t.Errorf("source = %v, want NWS", a.Source)
	}
	if a.Severity != api.AlertSeverity_CRITICAL {
		t.Errorf("severity = %v, want CRITICAL", a.Severity)
	}
	// Headline and Description are the two distinct authoritative fields.
	if a.Headline == "" || a.Description == "" {
		t.Error("headline/description should be populated")
	}
	if a.Headline == a.Description {
		t.Error("headline must not duplicate description")
	}
	// Summary/details are AI-enhancement slots and must be empty for NWS (no
	// 4x duplication of the same text).
	if a.Summary != "" {
		t.Errorf("summary should be empty for NWS, got %q", a.Summary)
	}
	if a.Details != "" {
		t.Errorf("details should be empty for NWS, got %q", a.Details)
	}
	if a.GetStartTime() == nil || a.GetEndTime() == nil {
		t.Error("start/end time should be set")
	}
}
