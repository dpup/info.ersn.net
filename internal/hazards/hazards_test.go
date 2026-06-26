package hazards

import (
	"context"
	"testing"
	"time"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/config"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPointGeom_SwapsAndTrims(t *testing.T) {
	g := PointGeom(38.0671234, -120.5402987) // internal {lat,lng}
	coords, ok := g.Coordinates.([]float64)
	if !ok || len(coords) != 2 {
		t.Fatalf("coordinates = %v", g.Coordinates)
	}
	// RFC 7946 order is [lon, lat], trimmed to 5 decimals.
	if coords[0] != -120.5403 {
		t.Errorf("lon = %v, want -120.5403 (trimmed)", coords[0])
	}
	if coords[1] != 38.06712 {
		t.Errorf("lat = %v, want 38.06712 (trimmed)", coords[1])
	}
	if g.Type != "Point" {
		t.Errorf("type = %q", g.Type)
	}
}

func TestLineStringAndPolygon(t *testing.T) {
	ls := LineStringGeom([]LatLng{{Lat: 38.0, Lng: -120.5}, {Lat: 38.1, Lng: -120.4}})
	if ls == nil || ls.Type != "LineString" {
		t.Fatalf("linestring = %+v", ls)
	}
	if LineStringGeom([]LatLng{{Lat: 38, Lng: -120}}) != nil {
		t.Error("single-point LineString should be nil")
	}

	poly := PolygonGeom([]LatLng{{Lat: 38, Lng: -120.5}, {Lat: 38.1, Lng: -120.4}, {Lat: 38, Lng: -120.3}})
	if poly == nil || poly.Type != "Polygon" {
		t.Fatalf("polygon = %+v", poly)
	}
	rings := poly.Coordinates.([][][]float64)
	ring := rings[0]
	if ring[0][0] != ring[len(ring)-1][0] || ring[0][1] != ring[len(ring)-1][1] {
		t.Error("polygon ring should be closed (first == last)")
	}
}

func TestSeverityMappings(t *testing.T) {
	cases := []struct {
		got  string
		want string
		rank int
	}{
		{fromAlertSeverity(api.AlertSeverity_CRITICAL), SevSevere, 3},
		{fromAlertSeverity(api.AlertSeverity_WARNING), SevModerate, 2},
		{fromAlertSeverity(api.AlertSeverity_INFO), SevMinor, 1},
		{fromAlertSeverity(api.AlertSeverity_ALERT_SEVERITY_UNSPECIFIED), SevInfo, 0},
		{fromChainLevelStr("R3"), SevSevere, 3},
		{fromChainLevelStr("R1"), SevMinor, 1},
		{fromFireWeatherState("red-flag"), SevSevere, 3},
		{fromFireWeatherState("NORMAL"), SevInfo, 0},
		{fromNWSSeverity("Extreme"), SevExtreme, 4},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("severity = %q, want %q", c.got, c.want)
		}
		if severityRank(c.got) != c.rank {
			t.Errorf("rank(%q) = %d, want %d", c.got, severityRank(c.got), c.rank)
		}
	}
}

func TestNormFireName(t *testing.T) {
	// CAL FIRE "Salt Springs Fire" and WFIGS "Salt Springs" must join.
	if normFireName("Salt Springs Fire") != normFireName("Salt Springs") {
		t.Errorf("%q != %q", normFireName("Salt Springs Fire"), normFireName("Salt Springs"))
	}
	if normFireName("Salt Springs Fire") != "saltsprings" {
		t.Errorf("got %q", normFireName("Salt Springs Fire"))
	}
}

func TestFromWildfire(t *testing.T) {
	cases := map[int32]string{0: SevSevere, 49: SevSevere, 50: SevModerate, 99: SevModerate, 100: SevMinor}
	for c, want := range cases {
		if got := fromWildfire(c); got != want {
			t.Errorf("fromWildfire(%d) = %q, want %q", c, got, want)
		}
	}
}

func TestSafeURL(t *testing.T) {
	if safeURL("https://protect.genasys.com/x") == "" {
		t.Error("https URL should pass")
	}
	if safeURL("javascript:alert(1)") != "" {
		t.Error("javascript: URL must be dropped")
	}
}

// --- fakes ---

type fakeRoads struct {
	incidents []*api.Incident
	roads     []*api.Road
}

func (f fakeRoads) ListRoads(context.Context, *api.ListRoadsRequest) (*api.ListRoadsResponse, error) {
	return &api.ListRoadsResponse{Roads: f.roads}, nil
}
func (f fakeRoads) ListIncidents(context.Context, *api.ListIncidentsRequest) (*api.ListIncidentsResponse, error) {
	return &api.ListIncidentsResponse{Incidents: f.incidents}, nil
}

func TestRoadIncidents_Reprojection(t *testing.T) {
	s := &Service{
		cfg: &config.Config{},
		roads: fakeRoads{incidents: []*api.Incident{{
			Id:                  "260625SA0982",
			Type:                api.AlertType_INCIDENT,
			Severity:            api.AlertSeverity_WARNING,
			Location:            &api.Coordinates{Latitude: 38.0671, Longitude: -120.5402},
			LocationDescription: "Sr49 / Monitor Rd",
			Description:         "Traffic Hazard",
			Status:              api.IncidentStatus_ACTIVE,
			LogNumber:           "260625SA0982",
			Started:             timestamppb.New(time.Unix(1782400000, 0)),
		}}},
	}
	feats, err := s.roadIncidents(context.Background(), config.HazardArea{IncidentArea: "mother-lode"})
	if err != nil {
		t.Fatal(err)
	}
	if len(feats) != 1 {
		t.Fatalf("got %d features, want 1", len(feats))
	}
	f := feats[0]
	if f.Properties.Layer != LayerRoadIncident {
		t.Errorf("layer = %q", f.Properties.Layer)
	}
	if f.Properties.Severity != SevModerate || f.Properties.SeverityRank != 2 {
		t.Errorf("severity = %q rank %d, want MODERATE/2", f.Properties.Severity, f.Properties.SeverityRank)
	}
	coords := f.Geometry.Coordinates.([]float64)
	if coords[0] != -120.5402 || coords[1] != 38.0671 {
		t.Errorf("coords = %v, want [-120.5402, 38.0671]", coords)
	}
	if f.Properties.Incident == nil || f.Properties.Incident.LogNumber != "260625SA0982" {
		t.Errorf("incident props = %+v", f.Properties.Incident)
	}
	if f.Properties.Status != "active" {
		t.Errorf("status = %q", f.Properties.Status)
	}
}
