# Hazard Aggregation (unified GeoJSON feed)

Implements the design in `docs/hazard-aggregation-design.md`. Aggregates the
service's hazard sources into ONE standardized, map-ready interface:

```
GET /api/v1/hazards/{area}/{layer}.geojson
```

Returns an RFC 7946 `FeatureCollection` (+ a `metadata` foreign member) that an
open maps client (MapLibre GL, Leaflet, OpenLayers) layers directly.

## The model (don't break the envelope)

- `geojson.go` — RFC 7946 types + geometry constructors. **Coordinates are
  `[longitude, latitude]`** (the inverse of the service's internal
  `{latitude, longitude}`); always build geometry via `PointGeom`/
  `LineStringGeom`/`PolygonGeom`, which do the swap and trim to 5 decimals.
- `properties.go` — the common `Properties` envelope shared by every layer, plus
  a namespaced per-kind block (`incident`, `road`, `chain_control`, `weather`,
  `fire_weather`). The envelope is identical across layers — that's the
  unification; a client renders any card from `headline/severity/source`.
- `severity.go` — the one severity scale (`INFO..EXTREME`, rank 0–4) every source
  maps onto. It's editorial response-urgency, not magnitude. Use `setSeverity`
  so `severity_rank` stays in sync.

## Served outside grpc-gateway, but CORS is automatic

These endpoints are hand-built GeoJSON via `prefab.WithHTTPHandler` (GeoJSON's
polymorphic geometry fights proto). Prefab still wraps every `WithHTTPHandler`
with `securityMiddleware` (verified in `builder.go`), so **CORS / the
`*.ersn.net` allowlist apply automatically — do not add manual `SecurityHeaders`
calls.** Each response sets `Content-Type: application/geo+json` and
`Cache-Control`.

## Fail-loud

If a layer's source errors, the handler returns `metadata.source_status =
UNAVAILABLE` with empty features — never a fabricated clear state. The evac layer
(M4) extends this: any empty active-evac result is `UNAVAILABLE`/`unknown`, never
"all clear".

`buildLayer` is the one place all this is enforced (both the single-layer and
`/situation` paths go through it). Status resolution:

- fresh cache hit → `OK` (no upstream call)
- builder OK → `OK` (non-empty result cached for `layerTTL`)
- builder returns `partialData(err)` → `STALE`, features kept (a multi-source
  layer like wildfire lost one source — don't present partial data as complete)
- builder hard error **with** a cached value → `STALE`, last-good features served
  (`last_source_update` = fetch time); transient upstream blips don't go dark
- builder hard error, nothing cached → `UNAVAILABLE`, empty
- empty active-events source (evac) → `UNAVAILABLE` (the fail-loud flip)

Caching uses the shared `internal/cache` (passed to `NewService`); `layerTTL`
returns 0 for the road/weather layers that are already cached by their underlying
services (no double-caching), and a short TTL for the new upstreams + the live
Caltrans chain-control fetch. Empty results are **never** cached (so an empty
all-clear can't be replayed as STALE).

## Adding a layer

1. Add the `layer` const in `properties.go` and a per-kind block struct.
2. Add the severity mapping in `severity.go` (cover every enum value).
3. Write a `builder` method `func (s *Service) <layer>(ctx, area) ([]Feature, error)`
   and add it to `layerRegistry()` (the single source of truth — it feeds BOTH
   the dispatch map and the `/situation` iteration order; there is no separate
   `layerOrder` to keep in sync). Give it a `layerTTL` if it hits a new upstream.
   A builder with multiple independent sources should return `partialData(err)`
   when one fails so the layer degrades to STALE, not a silent OK.
   **Scope to the area.** A builder MUST filter to the requested `area`, or a
   second configured area inherits the first's data. Use `area.Bounds.Contains`
   for geocoded sources (chain_control, earthquake, wildfire), the area's
   `incidentArea`/spatial query for incidents/evac, and `zonesMatch(area.Zones,
   …)` for the zone-based weather_alert / fire_weather layers (their data carries
   NWS zones, not coordinates). Returning everything regardless of `area` is the
   bug to avoid.
4. New upstreams get a client under `internal/clients/`, mirroring `nws`
   (HTTPDoer, no key where possible) and a `LimitReader` body cap.
5. M1 re-projects existing feeds only (road_incident, chain_control,
   road_segment, weather_alert null-geom, fire_weather null-geom). Roadmap:
   M2 earthquake (USGS) + scanners config; M3 wildfire (CAL FIRE + WFIGS
   perimeters); M4 evacuation (Cal OES, fail-loud); M5 `/situation/{area}`
   aggregator. Update the design doc's milestone table as each lands.

## /situation/{area} — the one-call rollup (M5)

`situation.go` fans out over every layer concurrently (`buildLayer` per layer,
same fail-loud rules as the GeoJSON endpoints) and returns a JSON summary, not
GeoJSON: per-layer `source_status` + `feature_count`, a cross-layer
`highest_severity`, severity histogram, the most-severe `top_headlines`, and the
scanner sidecar. The map still fetches `*.geojson` per layer; this is the
dashboard's single status fetch.

**Unknown-aware evacuation posture (don't regress):** `summary.active_evacuations`
is `null` whenever the evac layer is `UNAVAILABLE`, and `evacuation_status` says
which. A client MUST treat `null` as "unknown — check Genasys", never as zero.
Because the evac layer is `emptyUnavailable` (an empty active-events feed flips to
`UNAVAILABLE` in `buildLayer`), the production count is `null`-or-positive and
never `0`. The rollup math lives in the pure `summarize()` (unit-tested in
`situation_test.go`); its OK-with-zero branch returns `0` for completeness but is
unreachable for the evac layer through `buildLayer`.

Status: **M0–M5 shipped.** All eight layers + `/situation/{area}` +
`/scanners/{area}` are live. See the design doc's milestone table.
