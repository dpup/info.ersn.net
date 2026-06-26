package usgs

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeDoer struct {
	resp    string
	lastURL string
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.lastURL = req.URL.String()
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(f.resp)),
		Header:     make(http.Header),
	}, nil
}

const sample = `{
  "type": "FeatureCollection",
  "features": [
    {
      "id": "nc75095123",
      "properties": {
        "mag": 4.2,
        "place": "10km NE of Murphys, CA",
        "time": 1782400000000,
        "updated": 1782400500000,
        "felt": 37,
        "url": "https://earthquake.usgs.gov/earthquakes/eventpage/nc75095123"
      },
      "geometry": { "type": "Point", "coordinates": [-120.45, 38.2, 7.6] }
    },
    {
      "id": "nc75095124",
      "properties": { "mag": null, "place": "near nowhere", "time": 0, "felt": null, "url": "" },
      "geometry": { "type": "Point", "coordinates": [-120.5, 38.1, 3.0] }
    }
  ]
}`

func TestGetEarthquakes(t *testing.T) {
	doer := &fakeDoer{resp: sample}
	c := NewClientWithHTTPDoer("https://usgs.test", doer)

	quakes, err := c.GetEarthquakes(context.Background(),
		Bounds{MinLatitude: 37.8, MaxLatitude: 38.55, MinLongitude: -120.9, MaxLongitude: -120.0},
		2.5, 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(quakes) != 2 {
		t.Fatalf("got %d quakes, want 2", len(quakes))
	}
	q := quakes[0]
	if q.Magnitude != 4.2 || q.DepthKm != 7.6 {
		t.Errorf("mag/depth = %v/%v", q.Magnitude, q.DepthKm)
	}
	if q.Lat != 38.2 || q.Lng != -120.45 {
		t.Errorf("lat/lng = %v/%v", q.Lat, q.Lng)
	}
	if q.Felt != 37 {
		t.Errorf("felt = %d", q.Felt)
	}
	if q.Time.IsZero() {
		t.Error("time should be parsed from ms epoch")
	}
	// Second event: null mag / time 0 handled gracefully.
	if quakes[1].Magnitude != 0 || !quakes[1].Time.IsZero() {
		t.Errorf("null-field handling: mag=%v time=%v", quakes[1].Magnitude, quakes[1].Time)
	}

	// Query carries the bbox + format.
	for _, want := range []string{"format=geojson", "minlatitude=37.8", "maxlongitude=-120", "minmagnitude=2.5"} {
		if !strings.Contains(doer.lastURL, want) {
			t.Errorf("query missing %q: %s", want, doer.lastURL)
		}
	}
}
