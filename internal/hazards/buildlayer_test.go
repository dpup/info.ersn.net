package hazards

import (
	"context"
	"errors"
	"testing"

	"github.com/dpup/prefab/logging"

	"github.com/dpup/info.ersn.net/server/internal/cache"
	"github.com/dpup/info.ersn.net/server/internal/config"
)

// testCtx carries a logger so buildLayer's logging.Errorw/Warnw don't panic.
func testCtx() context.Context { return logging.EnsureLogger(context.Background()) }

func okBuild(fs ...Feature) builder {
	return func(context.Context, config.HazardArea) ([]Feature, error) { return fs, nil }
}
func errBuild(err error) builder {
	return func(context.Context, config.HazardArea) ([]Feature, error) { return nil, err }
}

// TestBuildLayer_FreshCacheHit: a second request inside the TTL is served from
// cache without invoking the builder again.
func TestBuildLayer_FreshCacheHit(t *testing.T) {
	s := &Service{cache: cache.NewCache()}
	area := config.HazardArea{ID: "x"}
	r1 := s.buildLayer(testCtx(), area, LayerEarthquake, okBuild(feat(SevMinor, "q")))
	if r1.status != "OK" || len(r1.features) != 1 {
		t.Fatalf("first build = %q/%d", r1.status, len(r1.features))
	}
	// Builder now fails the test if called — the fresh cache must satisfy this.
	poison := func(context.Context, config.HazardArea) ([]Feature, error) {
		t.Error("builder should not be called on a fresh cache hit")
		return nil, nil
	}
	r2 := s.buildLayer(testCtx(), area, LayerEarthquake, poison)
	if r2.status != "OK" || len(r2.features) != 1 {
		t.Fatalf("cached build = %q/%d", r2.status, len(r2.features))
	}
}

// TestBuildLayer_StaleOnError: when the upstream fails but a (stale) cached value
// exists, the layer degrades to STALE serving last-good data — not UNAVAILABLE.
func TestBuildLayer_StaleOnError(t *testing.T) {
	s := &Service{cache: cache.NewCache()}
	area := config.HazardArea{ID: "x"}
	key := "hazard:x:" + LayerEarthquake
	// Inject an already-stale entry (TTL 0 => ExpiresAt == now).
	if err := s.cache.Set(key, []Feature{feat(SevModerate, "old")}, 0, "test"); err != nil {
		t.Fatal(err)
	}
	r := s.buildLayer(testCtx(), area, LayerEarthquake, errBuild(errors.New("boom")))
	if r.status != "STALE" {
		t.Fatalf("status = %q, want STALE", r.status)
	}
	if len(r.features) != 1 {
		t.Fatalf("stale features = %d, want 1 (last good)", len(r.features))
	}
	if r.lastSourceUpdate.IsZero() {
		t.Error("STALE result must carry last_source_update")
	}
}

// TestBuildLayer_UnavailableNoCache: upstream fails with nothing cached => fail-loud.
func TestBuildLayer_UnavailableNoCache(t *testing.T) {
	s := &Service{cache: cache.NewCache()}
	r := s.buildLayer(testCtx(), config.HazardArea{ID: "x"}, LayerEarthquake, errBuild(errors.New("boom")))
	if r.status != "UNAVAILABLE" || len(r.features) != 0 {
		t.Fatalf("got %q/%d, want UNAVAILABLE/0", r.status, len(r.features))
	}
}

// TestBuildLayer_PartialIsStale: a builder returning partialData keeps its
// (incomplete) features and reports STALE, not a silent OK.
func TestBuildLayer_PartialIsStale(t *testing.T) {
	s := &Service{cache: cache.NewCache()}
	partial := func(context.Context, config.HazardArea) ([]Feature, error) {
		return []Feature{feat(SevSevere, "one source")}, partialData(errors.New("other source down"))
	}
	r := s.buildLayer(testCtx(), config.HazardArea{ID: "x"}, LayerWildfire, partial)
	if r.status != "STALE" || len(r.features) != 1 {
		t.Fatalf("got %q/%d, want STALE/1", r.status, len(r.features))
	}
}

// TestBuildLayer_EvacEmptyUnavailable: an empty active-events evac result stays
// fail-loud (UNAVAILABLE) and is NOT cached as an all-clear.
func TestBuildLayer_EvacEmptyUnavailable(t *testing.T) {
	s := &Service{cache: cache.NewCache()}
	r := s.buildLayer(testCtx(), config.HazardArea{ID: "x"}, LayerEvacuation, okBuild())
	if r.status != "UNAVAILABLE" {
		t.Fatalf("empty evac status = %q, want UNAVAILABLE", r.status)
	}
	// The empty result must not have been cached (no all-clear replay).
	var anything []Feature
	if ok, _ := s.cache.Get("hazard:x:"+LayerEvacuation, &anything); ok {
		t.Error("empty evac result must not be cached")
	}
}
