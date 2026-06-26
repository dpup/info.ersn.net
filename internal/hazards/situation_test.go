package hazards

import "testing"

// TestLayerRegistryUnique guards the single source of truth: the registry that
// feeds both the dispatch map and the situation order must have no duplicate
// names and a non-nil builder per entry.
func TestLayerRegistryUnique(t *testing.T) {
	s := &Service{}
	seen := map[string]bool{}
	for _, e := range s.layerRegistry() {
		if e.name == "" || e.build == nil {
			t.Errorf("registry entry %+v has empty name or nil builder", e)
		}
		if seen[e.name] {
			t.Errorf("duplicate layer in registry: %q", e.name)
		}
		seen[e.name] = true
	}
}

func feat(sev string, headline string) Feature {
	p := Properties{Headline: headline, Source: Source{Name: "Test"}}
	p.setSeverity(sev)
	return Feature{Type: "Feature", Properties: p}
}

// TestSummarize_EvacUnknownAware is the load-bearing case: when the evacuation
// layer is UNAVAILABLE, the count must be nil ("unknown"), never 0/all-clear.
func TestSummarize_EvacUnknownAware(t *testing.T) {
	results := []namedResult{
		{layer: LayerEvacuation, res: layerResult{status: "UNAVAILABLE", features: nil}},
		{layer: LayerRoadIncident, res: layerResult{status: "OK", features: []Feature{
			feat(SevModerate, "Collision"),
			feat(SevInfo, "Debris"),
		}}},
	}
	sum, layers := summarize(results)

	if sum.ActiveEvacuations != nil {
		t.Errorf("active_evacuations = %v, want nil (unknown) when evac UNAVAILABLE", *sum.ActiveEvacuations)
	}
	if sum.EvacuationStatus != "UNAVAILABLE" {
		t.Errorf("evacuation_status = %q", sum.EvacuationStatus)
	}
	if sum.HighestSeverity != SevModerate || sum.HighestSeverityRank != 2 {
		t.Errorf("highest = %q/%d, want MODERATE/2", sum.HighestSeverity, sum.HighestSeverityRank)
	}
	if sum.TotalFeatures != 2 || sum.SeverityCounts[SevModerate] != 1 || sum.SeverityCounts[SevInfo] != 1 {
		t.Errorf("counts = %+v total=%d", sum.SeverityCounts, sum.TotalFeatures)
	}
	if len(layers) != 2 {
		t.Fatalf("layers = %d, want 2", len(layers))
	}
}

// TestSummarize_EvacKnownZero: when Cal OES answers with zero active zones, the
// count is a real 0 (not nil) — the source is healthy and says "none right now".
func TestSummarize_EvacKnownZero(t *testing.T) {
	sum, _ := summarize([]namedResult{
		{layer: LayerEvacuation, res: layerResult{status: "OK", features: nil}},
	})
	if sum.ActiveEvacuations == nil {
		t.Fatal("active_evacuations should be a real 0 when evac source is OK")
	}
	if *sum.ActiveEvacuations != 0 {
		t.Errorf("active_evacuations = %d, want 0", *sum.ActiveEvacuations)
	}
}

func TestSummarize_TopHeadlinesSorted(t *testing.T) {
	sum, _ := summarize([]namedResult{
		{layer: LayerWeatherAlert, res: layerResult{status: "OK", features: []Feature{
			feat(SevInfo, "low"),
			feat(SevExtreme, "top"),
			feat(SevModerate, "mid"),
		}}},
	})
	if len(sum.TopHeadlines) != 3 || sum.TopHeadlines[0].Headline != "top" {
		t.Fatalf("headlines not sorted most-severe-first: %+v", sum.TopHeadlines)
	}
	if sum.TopHeadlines[2].Headline != "low" {
		t.Errorf("least severe should be last: %+v", sum.TopHeadlines)
	}
}

// TestSummarize_EmptyDefaultsInfo: no features anywhere => INFO, not "".
func TestSummarize_EmptyDefaultsInfo(t *testing.T) {
	sum, _ := summarize([]namedResult{
		{layer: LayerEarthquake, res: layerResult{status: "OK", features: nil}},
	})
	if sum.HighestSeverity != SevInfo {
		t.Errorf("highest_severity = %q, want INFO when no features", sum.HighestSeverity)
	}
}
