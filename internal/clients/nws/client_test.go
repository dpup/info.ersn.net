package nws

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeDoer struct {
	resp       string
	status     int
	lastURL    string
	lastUA     string
	lastAccept string
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.lastURL = req.URL.String()
	f.lastUA = req.Header.Get("User-Agent")
	f.lastAccept = req.Header.Get("Accept")
	status := f.status
	if status == 0 {
		status = 200
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(f.resp)),
		Header:     make(http.Header),
	}, nil
}

const sampleGeoJSON = `{
  "features": [
    {
      "properties": {
        "id": "urn:oid:2.49.0.1.840.0.abc",
        "event": "Red Flag Warning",
        "severity": "Severe",
        "certainty": "Likely",
        "urgency": "Expected",
        "headline": "Red Flag Warning in effect",
        "description": "Gusty winds and low humidity.",
        "instruction": "Avoid outdoor burning.",
        "senderName": "NWS Sacramento CA",
        "areaDesc": "Calaveras",
        "effective": "2026-06-26T10:00:00-07:00",
        "expires": "2026-06-27T20:00:00-07:00",
        "geocode": { "UGC": ["CAZ064", "CAZ065"] }
      }
    },
    {
      "properties": {
        "id": "urn:oid:2.49.0.1.840.0.def",
        "event": "Heat Advisory",
        "severity": "Moderate",
        "headline": "Heat Advisory",
        "description": "Hot.",
        "senderName": "NWS Sacramento CA",
        "geocode": { "UGC": ["CAZ258"] }
      }
    }
  ]
}`

func TestGetActiveZoneAlerts(t *testing.T) {
	doer := &fakeDoer{resp: sampleGeoJSON}
	c := NewClientWithHTTPDoer("test-agent", "https://nws.test", doer)

	alerts, err := c.GetActiveZoneAlerts(context.Background(), []string{"CAZ064", "CAZ065", "CAZ258", "CAZ259"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("got %d alerts, want 2", len(alerts))
	}

	rf := alerts[0]
	if rf.Event != "Red Flag Warning" {
		t.Errorf("event = %q", rf.Event)
	}
	if rf.Severity != "Severe" {
		t.Errorf("severity = %q", rf.Severity)
	}
	if len(rf.Zones) != 2 || rf.Zones[0] != "CAZ064" {
		t.Errorf("zones = %v", rf.Zones)
	}
	if rf.Effective.IsZero() {
		t.Error("expected effective time parsed")
	}

	// Request headers and zone param
	if doer.lastUA != "test-agent" {
		t.Errorf("User-Agent = %q", doer.lastUA)
	}
	if !strings.Contains(doer.lastAccept, "geo+json") {
		t.Errorf("Accept = %q", doer.lastAccept)
	}
	if !strings.Contains(doer.lastURL, "zone=CAZ064%2CCAZ065%2CCAZ258%2CCAZ259") {
		t.Errorf("URL missing zone param: %q", doer.lastURL)
	}
}

func TestGetActiveZoneAlerts_NoZones(t *testing.T) {
	doer := &fakeDoer{resp: sampleGeoJSON}
	c := NewClientWithHTTPDoer("test-agent", "https://nws.test", doer)
	alerts, err := c.GetActiveZoneAlerts(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alerts != nil {
		t.Errorf("expected nil alerts for empty zones, got %v", alerts)
	}
	if doer.lastURL != "" {
		t.Errorf("expected no request to be made, got %q", doer.lastURL)
	}
}

func TestCleanZones(t *testing.T) {
	got := cleanZones([]string{" caz064 ", "CAZ064", "CAZ065,CAZ258", ""})
	want := []string{"CAZ064", "CAZ065", "CAZ258"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
