package hazards

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dpup/prefab/logging"

	"github.com/dpup/info.ersn.net/server/internal/config"
)

// SituationPrefix mounts the cross-layer situation aggregator.
const SituationPrefix = "/api/v1/situation/"

// Situation is the one-call rollup for an area: every hazard layer's status,
// a severity summary, the evacuation posture (unknown-aware), and the scanner
// sidecar. It's a dashboard's single fetch — GeoJSON layers are still fetched
// per-layer for the map.
type Situation struct {
	Area        string           `json:"area"`
	AreaName    string           `json:"area_name,omitempty"`
	GeneratedAt string           `json:"generated_at"`
	Summary     SituationSummary `json:"summary"`
	Layers      []LayerStatus    `json:"layers"`
	Scanners    []scannerOut     `json:"scanners"`
}

// SituationSummary is the at-a-glance rollup across all layers.
type SituationSummary struct {
	HighestSeverity     string         `json:"highest_severity"`
	HighestSeverityRank int            `json:"highest_severity_rank"`
	SeverityCounts      map[string]int `json:"severity_counts"`
	TotalFeatures       int            `json:"total_features"`
	// ActiveEvacuations is the count of active evacuation zones, or null when
	// the Cal OES source is unavailable — a client MUST render null as "unknown"
	// (check the road), never as zero/all-clear. EvacuationStatus disambiguates.
	ActiveEvacuations *int   `json:"active_evacuations"`
	EvacuationStatus  string `json:"evacuation_status"`
	// TopHeadlines lists the most severe features first (for a banner/teaser).
	TopHeadlines []Headline `json:"top_headlines"`
}

// Headline is a compact, source-attributed teaser for the most urgent hazards.
type Headline struct {
	Layer        string `json:"layer"`
	Severity     string `json:"severity"`
	SeverityRank int    `json:"severity_rank"`
	Headline     string `json:"headline"`
	Source       string `json:"source,omitempty"`
}

// LayerStatus is one layer's contribution to the situation rollup.
type LayerStatus struct {
	Layer           string `json:"layer"`
	SourceStatus    string `json:"source_status"`
	FeatureCount    int    `json:"feature_count"`
	HighestSeverity string `json:"highest_severity,omitempty"`
	Attribution     string `json:"attribution,omitempty"`
	SourceURL       string `json:"source_url,omitempty"`
}

// ServeSituation handles GET /api/v1/situation/{area} — a concurrent fan-out
// over every layer.
func (s *Service) ServeSituation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	areaID := strings.Trim(strings.TrimPrefix(r.URL.Path, SituationPrefix), "/")
	area, ok := s.resolveArea(areaID)
	if !ok {
		http.Error(w, "unknown hazard area: "+areaID, http.StatusNotFound)
		return
	}

	// Bound the whole fan-out: each upstream client has its own timeout, but
	// without an aggregate deadline a single slow source holds the handler for
	// the sum of those timeouts.
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	builders := s.builders()

	// Fan out: every layer builds concurrently. A slow/broken source becomes
	// UNAVAILABLE for its layer without stalling or failing the whole rollup.
	results := make([]namedResult, len(layerOrder))
	var wg sync.WaitGroup
	for i, layer := range layerOrder {
		b, ok := builders[layer]
		if !ok {
			continue
		}
		wg.Add(1)
		go func(i int, layer string, b builder) {
			defer wg.Done()
			results[i] = namedResult{layer: layer, res: s.buildLayer(ctx, area, layer, b)}
		}(i, layer, b)
	}
	wg.Wait()

	summary, layers := summarize(results)
	out := Situation{
		Area:        area.ID,
		AreaName:    area.Name,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Summary:     summary,
		Layers:      layers,
		Scanners:    s.scanners(area),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		logging.Errorw(ctx, "Failed to encode situation rollup", "error", err)
	}
}

// namedResult pairs a layer name with its build result (input to summarize).
type namedResult struct {
	layer string
	res   layerResult
}

// summarize rolls per-layer results into the situation summary + per-layer
// statuses. Pure (no I/O) so the unknown-aware evacuation logic and the
// severity rollup are unit-testable without touching the network.
func summarize(results []namedResult) (SituationSummary, []LayerStatus) {
	summary := SituationSummary{
		SeverityCounts:   map[string]int{},
		EvacuationStatus: "UNAVAILABLE",
	}
	layers := make([]LayerStatus, 0, len(results))
	for _, r := range results {
		if r.layer == "" {
			continue
		}
		ls := LayerStatus{
			Layer:        r.layer,
			SourceStatus: r.res.status,
			FeatureCount: len(r.res.features),
			Attribution:  r.res.meta.attribution,
			SourceURL:    r.res.meta.sourceURL,
		}
		layerTop := -1
		for _, f := range r.res.features {
			sev := f.Properties.Severity
			rank := f.Properties.SeverityRank
			summary.SeverityCounts[sev]++
			summary.TotalFeatures++
			if rank > layerTop {
				layerTop = rank
				ls.HighestSeverity = sev
			}
			if rank > summary.HighestSeverityRank || summary.HighestSeverity == "" {
				summary.HighestSeverityRank = rank
				summary.HighestSeverity = sev
			}
			summary.TopHeadlines = append(summary.TopHeadlines, Headline{
				Layer:        r.layer,
				Severity:     sev,
				SeverityRank: rank,
				Headline:     f.Properties.Headline,
				Source:       f.Properties.Source.Name,
			})
		}
		// Evacuation posture is unknown-aware: only report a real count when Cal
		// OES answered. While UNAVAILABLE, ActiveEvacuations stays nil ("unknown").
		if r.layer == LayerEvacuation {
			summary.EvacuationStatus = r.res.status
			if r.res.status == "OK" {
				n := len(r.res.features)
				summary.ActiveEvacuations = &n
			}
		}
		layers = append(layers, ls)
	}

	if summary.HighestSeverity == "" {
		summary.HighestSeverity = SevInfo
	}
	summary.TopHeadlines = topHeadlines(summary.TopHeadlines, 5)
	return summary, layers
}

// scanners builds the scanner sidecar (same shape as ServeScanners).
func (s *Service) scanners(area config.HazardArea) []scannerOut {
	out := make([]scannerOut, 0, len(area.ScannerFeeds))
	for _, f := range area.ScannerFeeds {
		out = append(out, scannerOut{
			FeedID:          f.FeedID,
			ChannelLabel:    f.ChannelLabel,
			Agency:          f.Agency,
			BroadcastifyURL: "https://www.broadcastify.com/listen/feed/" + f.FeedID,
		})
	}
	return out
}

// topHeadlines returns the n most severe headlines, most urgent first, stable on
// ties (preserves the layer order they were collected in).
func topHeadlines(h []Headline, n int) []Headline {
	sort.SliceStable(h, func(i, j int) bool { return h[i].SeverityRank > h[j].SeverityRank })
	if len(h) > n {
		h = h[:n]
	}
	return h
}

// layerOrder fixes the layer iteration order for the rollup (deterministic
// output; map iteration is random). Mirrors builders().
var layerOrder = []string{
	LayerEvacuation,
	LayerWildfire,
	LayerRoadIncident,
	LayerChainControl,
	LayerWeatherAlert,
	LayerFireWeather,
	LayerEarthquake,
	LayerRoadSegment,
}
