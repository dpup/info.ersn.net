package services

import (
	"strings"
	"testing"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/clients/caltrans"
	"github.com/dpup/info.ersn.net/server/internal/config"
)

func motherLode() config.IncidentArea {
	return config.IncidentArea{
		ID:   "mother-lode",
		Name: "Mother Lode",
		Bounds: config.GeoBounds{
			MinLatitude:  37.3,
			MaxLatitude:  39.0,
			MinLongitude: -121.15,
			MaxLongitude: -119.5,
		},
	}
}

// chpDescription mirrors the real quickmap CHP CDATA structure.
const chpDescription = `
<div style="font-size:1.15em;"><img src="x" style="float:left"><p align="left">Sep 16 2025  8:36AM <br> 1182-Trfc Collision-No Inj <br> Hwy 49 / Parrotts Ferry Rd </p>
<p align="left">Sep 16 2025  8:37AM [1] UNITS EN ROUTE <br /> </p><p>Information courtesy of CHP</p>
<p class="update-stamp">Last updated: 09/16/2025 9:17am </p></div>`

func TestBuildIncident_CHPParsing(t *testing.T) {
	s := &RoadsService{}
	in := caltrans.CaltransIncident{
		FeedType:        caltrans.CHP_INCIDENT,
		Name:            "CHP Incident 250916ST0066",
		DescriptionHtml: chpDescription,
		DescriptionText: "Trfc Collision",
		StyleUrl:        "#chp",
		Coordinates:     &api.Coordinates{Latitude: 38.0671, Longitude: -120.5402}, // Angels Camp
	}

	inc := s.buildIncident(in, motherLode())
	if inc == nil {
		t.Fatal("expected incident, got nil")
	}
	if inc.LogNumber != "250916ST0066" {
		t.Errorf("log number = %q, want 250916ST0066", inc.LogNumber)
	}
	if inc.Id != "250916ST0066" {
		t.Errorf("id = %q, want 250916ST0066", inc.Id)
	}
	if inc.Type != api.AlertType_INCIDENT {
		t.Errorf("type = %v, want INCIDENT", inc.Type)
	}
	if inc.LocationDescription != "Hwy 49 / Parrotts Ferry Rd" {
		t.Errorf("location = %q, want 'Hwy 49 / Parrotts Ferry Rd'", inc.LocationDescription)
	}
	if inc.Description != "1182-Trfc Collision-No Inj" {
		t.Errorf("description = %q", inc.Description)
	}
	if inc.Severity != api.AlertSeverity_WARNING {
		t.Errorf("severity = %v, want WARNING (collision)", inc.Severity)
	}
	if inc.Started == nil {
		t.Error("expected Started to be parsed")
	}
	if inc.LastUpdated == nil {
		t.Error("expected LastUpdated to be parsed")
	}
	if inc.Area != "mother-lode" {
		t.Errorf("area = %q", inc.Area)
	}
}

func TestBuildIncident_OutsideBoundsExcluded(t *testing.T) {
	s := &RoadsService{}
	in := caltrans.CaltransIncident{
		FeedType:    caltrans.CHP_INCIDENT,
		Name:        "CHP Incident 250916ST0099",
		Coordinates: &api.Coordinates{Latitude: 38.4951, Longitude: -121.4413}, // Sacramento (Central Valley)
	}
	if inc := s.buildIncident(in, motherLode()); inc != nil {
		t.Errorf("expected nil for out-of-bounds incident, got %+v", inc)
	}
}

func TestBuildIncident_NilCoordinates(t *testing.T) {
	s := &RoadsService{}
	in := caltrans.CaltransIncident{FeedType: caltrans.CHP_INCIDENT, Name: "no coords"}
	if inc := s.buildIncident(in, motherLode()); inc != nil {
		t.Error("expected nil for incident without coordinates")
	}
}

func TestIncidentSeverity(t *testing.T) {
	tests := []struct {
		name     string
		feed     caltrans.CaltransFeedType
		typeText string
		style    string
		want     api.AlertSeverity
	}{
		{"injury collision", caltrans.CHP_INCIDENT, "1183-Trfc Collision-Injury", "#chp", api.AlertSeverity_CRITICAL},
		{"no-injury collision", caltrans.CHP_INCIDENT, "1182-Trfc Collision-No Inj", "#chp", api.AlertSeverity_WARNING},
		{"assist", caltrans.CHP_INCIDENT, "Assist CT with Maintenance", "#chp", api.AlertSeverity_INFO},
		{"lane closure", caltrans.LANE_CLOSURE, "", "#closure", api.AlertSeverity_WARNING},
		{"full closure", caltrans.LANE_CLOSURE, "", "#full-closure", api.AlertSeverity_CRITICAL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := caltrans.CaltransIncident{FeedType: tt.feed, StyleUrl: tt.style}
			got := incidentSeverity(in, tt.typeText)
			if got != tt.want {
				t.Errorf("severity = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractLogNumber(t *testing.T) {
	cases := map[string]string{
		"CHP Incident 250916ST0066": "250916ST0066",
		"CHP Incident 250911GG0206": "250911GG0206",
		"Some Lane Closure":         "",
	}
	for name, want := range cases {
		in := caltrans.CaltransIncident{Name: name}
		if got := extractLogNumber(in, ""); got != want {
			t.Errorf("extractLogNumber(%q) = %q, want %q", name, got, want)
		}
	}
}

// chpDescription2026 mirrors the 2026 quickmap "infowindow" CHP markup, where
// <name> is blank and details live in iw-* elements.
const chpDescription2026 = `<div class="infowindow-content">
  <div class="iw-header"><div class="iw-header-left">
    <img class="iw-icon" src="x" /> CHP Incident 260625SA1034
  </div></div>
  <div class="iw-body">
    <h2 class="iw-title">1183-Trfc Collision-Injury</h2>
    <p class="iw-text">Jun 25 2026  6:24PM <br> Hwy 49 / Parrotts Ferry Rd</p>
    <p class="iw-text">Jun 25 2026  6:20PM [2] units en route<br /></p>
    <p class="iw-attribution">Information courtesy of <strong>CHP</strong></p>
  </div>
  <div class="iw-footer"><span class="iw-timestamp">Last updated: <strong>06/25/2026</strong> 6:27pm</span></div>
</div>`

func TestBuildIncident_CHP2026Format(t *testing.T) {
	s := &RoadsService{}
	// Name blank as in the live feed - relies on description parsing.
	in := caltrans.CaltransIncident{
		FeedType:        caltrans.CHP_INCIDENT,
		Name:            "CHP Incident 260625SA1034", // backfilled by the client
		DescriptionHtml: chpDescription2026,
		Coordinates:     &api.Coordinates{Latitude: 38.0671, Longitude: -120.5402},
	}
	inc := s.buildIncident(in, motherLode())
	if inc == nil {
		t.Fatal("expected incident")
	}
	if inc.LogNumber != "260625SA1034" {
		t.Errorf("log = %q, want 260625SA1034", inc.LogNumber)
	}
	if inc.Description != "1183-Trfc Collision-Injury" {
		t.Errorf("description = %q", inc.Description)
	}
	if inc.LocationDescription != "Hwy 49 / Parrotts Ferry Rd" {
		t.Errorf("location = %q", inc.LocationDescription)
	}
	if inc.Severity != api.AlertSeverity_CRITICAL {
		t.Errorf("severity = %v, want CRITICAL (injury)", inc.Severity)
	}
	if inc.Started == nil {
		t.Error("expected Started parsed from iw-text")
	}
	if inc.LastUpdated == nil {
		t.Error("expected LastUpdated parsed from iw-timestamp")
	}
}

// laneClosure2026 mirrors the 2026 lane-closure markup.
const laneClosure2026 = `<div class="infowindow-content">
  <div class="iw-header"><div class="iw-header-left"><img class="iw-icon" src="x" /> Lane Closure</div></div>
  <div class="iw-body">
    <h2 class="iw-title">Route 4 One-way Traffic Operation</h2>
    <p class="iw-text">From 0.5 mi E of Murphys to 0.8 mi E / Expect 20-minute delays</p>
    <p class="iw-text"> Due to Emergency Work</p>
    <div style='font-size:xx-small;'>Closure ID: C4TA, Log Number: 42</div>
  </div>
</div>`

func TestBuildIncident_LaneClosure2026Format(t *testing.T) {
	s := &RoadsService{}
	in := caltrans.CaltransIncident{
		FeedType:        caltrans.LANE_CLOSURE,
		Name:            "Route 4 One-way Traffic Operation",
		DescriptionHtml: laneClosure2026,
		Coordinates:     &api.Coordinates{Latitude: 38.139, Longitude: -120.456},
	}
	inc := s.buildIncident(in, motherLode())
	if inc == nil {
		t.Fatal("expected incident")
	}
	if inc.Type != api.AlertType_CLOSURE {
		t.Errorf("type = %v, want CLOSURE", inc.Type)
	}
	if inc.LogNumber != "C4TA" {
		t.Errorf("log = %q, want C4TA", inc.LogNumber)
	}
	if inc.Description != "Route 4 One-way Traffic Operation" {
		t.Errorf("description = %q", inc.Description)
	}
	if !strings.Contains(inc.LocationDescription, "Murphys") {
		t.Errorf("location = %q, want to contain Murphys", inc.LocationDescription)
	}
	if inc.Severity != api.AlertSeverity_WARNING {
		t.Errorf("severity = %v, want WARNING", inc.Severity)
	}
}
