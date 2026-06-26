package caloes

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeDoer struct {
	resp    string
	lastURL string
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.lastURL = req.URL.String()
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(f.resp)), Header: make(http.Header)}, nil
}

const sample = `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "ZONE_ID": "CAL-E063",
        "ZONE_NAME": "Hathaway Pines",
        "COUNTY": "Calaveras",
        "STATUS": "Evacuation Order",
        "EVENT_TYPE": "Fire",
        "PUBLIC_INFO": "Leave now.",
        "STATEWIDE_LAST_UPDATED": 1782400000000
      },
      "geometry": { "type": "Polygon", "coordinates": [[[-120.4,38.2],[-120.3,38.2],[-120.3,38.3],[-120.4,38.2]]] }
    }
  ]
}`

func TestGetActiveEvacuations(t *testing.T) {
	doer := &fakeDoer{resp: sample}
	c := NewClientWithHTTPDoer("https://caloes.test/query", doer)

	zones, err := c.GetActiveEvacuations(context.Background(), []string{"Calaveras", "Tuolumne"})
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 1 {
		t.Fatalf("got %d, want 1", len(zones))
	}
	z := zones[0]
	if z.ZoneID != "CAL-E063" || z.Status != "Evacuation Order" || z.County != "Calaveras" {
		t.Errorf("zone = %+v", z)
	}
	if z.LastUpdated.IsZero() {
		t.Error("last updated should parse from ms epoch")
	}
	if z.GeometryType != "Polygon" || len(z.GeometryCoords) == 0 {
		t.Errorf("geometry = %s / %s", z.GeometryType, z.GeometryCoords)
	}
	// County filter is applied in the WHERE clause (quoted, comma-joined).
	if !strings.Contains(doer.lastURL, "COUNTY+IN") {
		t.Errorf("query missing county filter: %s", doer.lastURL)
	}
}

func TestCountyWhere(t *testing.T) {
	if got := countyWhere([]string{"Calaveras", "O'Brien"}); got != "COUNTY IN ('Calaveras','O''Brien')" {
		t.Errorf("countyWhere = %q", got)
	}
	if got := countyWhere(nil); got != "1=1" {
		t.Errorf("empty countyWhere = %q", got)
	}
}
