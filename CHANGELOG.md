# Changelog

All notable **API-facing** changes to the ERSN Info Server. This is the document
to read before updating a consuming site (e.g. ersn.net, sierragridteam.org).

There are no formal releases — the service deploys from `main`. Each entry below
is timestamped; add a new dated section at the top when the API surface changes.
The API is JSON over HTTP (`/api/v1/...`); field names are camelCase.

## 2026-06-27 00:30 UTC

### Changed — hazard layer resilience + contract polish (code-review follow-up)

Hardening pass on the M1–M5 hazard endpoints. All additive/clarifying — no
consumer of the (still-unreleased) hazard API needs to change, but new fields and
states are now observable:

- **`source_status: STALE` is now emitted** (previously only OK/UNAVAILABLE). A
  layer reports `STALE` when it is serving the last good fetch after a transient
  upstream failure, or when one of a layer's multiple sources failed (e.g. CAL
  FIRE up but WFIGS down). `metadata.last_source_update` (and, in `/situation`,
  `layers[].last_source_update`) carries the RFC3339 time of that last good fetch.
- **`/situation` `active_evacuations` now also reports a count when evacuation
  data is `STALE`** (served from cache), with `evacuation_status: "STALE"`. It
  remains `null` only when truly `UNAVAILABLE`. Still never `0`.
- **`road_segment` numeric fields (`delay_minutes`, `duration_minutes`,
  `distance_km`) are now omitted when a segment has no live data yet**, instead of
  serializing `0`. A present `0` now unambiguously means a real zero (e.g. no
  delay). `congestion`/`status` were already omitted when absent.
- Evacuation zones are now selected by the area's geographic bounds (ArcGIS
  spatial query) rather than a county-name list, so an in-area zone tagged to a
  neighboring county is no longer dropped. No response-shape change.
- Internal: the new upstreams (USGS, CAL FIRE, WFIGS, Cal OES) and the live
  Caltrans chain-control fetch are now server-side cached (2–5 min TTL) with
  stale-on-error fallback; a burst of map clients no longer fans out to every
  source on every request.

## 2026-06-26 23:55 UTC

### Added — hazard layers M2–M5 (earthquake, wildfire, evacuation, situation rollup)

Completes the hazard aggregation roadmap. All additive — existing endpoints and
the M1 layers are unchanged.

New GeoJSON layers at `GET /api/v1/hazards/{area}/{layer}.geojson`:

- `earthquake` — USGS events (M≥2.5, last 7 days) within the area bounds, as
  Points with `properties.earthquake` (`magnitude, depth_km, felt`).
- `wildfire` — CAL FIRE active incidents joined to NIFC/WFIGS perimeters by fire
  name. Polygon where a perimeter exists, else a Point; `properties.wildfire`
  (`acres, containment, county, has_perimeter`).
- `evacuation` — Cal OES active evacuation zones (Order/Warning/Advisory/SIP) as
  Polygons; `properties.evacuation` (`zone_id, level, event_type, county`).
  **Fail-loud / life-safety:** this is an active-events-only source, so an empty
  result is `metadata.source_status = UNAVAILABLE` (never an implied all-clear),
  and `metadata.source_url` always links the authoritative Genasys viewer
  (`protect.genasys.com`). Attribution is "reference only".

New JSON (non-GeoJSON) endpoints:

- `GET /api/v1/situation/{area}` — one-call rollup for a dashboard: per-layer
  `source_status` + `feature_count`, a cross-layer `summary` (`highest_severity`,
  `severity_counts`, `top_headlines`), and a `scanners` sidecar.
  **`summary.active_evacuations` is `null` when evacuation data is unavailable**
  (`summary.evacuation_status` says which) — render `null` as "unknown", never as
  zero. A real `0` only appears when Cal OES answered with no active zones.
- `GET /api/v1/scanners/{area}` — Broadcastify scanner feeds for the area
  (`feed_id, channel_label, agency, broadcastify_url`). Link-out only; no embed.

## 2026-06-26 22:28 UTC

### Added — unified hazard GeoJSON feed (M1)

New map-ready endpoints aggregating hazard data into one standardized RFC 7946
GeoJSON interface (see `docs/hazard-aggregation-design.md`):

```
GET /api/v1/hazards/{area}/{layer}.geojson
```

- Areas are configured under `hazards.areas` in `prefab.yaml` (first: `calaveras`).
- M1 layers (re-project existing feeds): `road_incident`, `chain_control`,
  `road_segment` (LineString), `weather_alert` (null-geometry banner),
  `fire_weather` (null-geometry banner). Roadmap: `earthquake`, `wildfire`,
  `evacuation`, and a `/situation/{area}` aggregator.
- Every feature uses a common `properties` envelope (`id, layer, kind, severity,
  severity_rank, headline, source, …`) + a namespaced per-kind block, and a
  unified severity scale `INFO..EXTREME` (rank 0–4) for sort/color.
- Coordinates are RFC 7946 `[longitude, latitude]`. Collections carry a
  `metadata` member with `source_status` (OK/STALE/UNAVAILABLE) for fail-loud
  provenance. `Content-Type: application/geo+json`.
- Consumes directly in MapLibre GL / Leaflet (`addSource({type:'geojson'})`).

This is additive — existing endpoints are unchanged.

## 2026-06-26 16:47 UTC

A large API cleanup pass. Several responses changed shape — see **Breaking
changes** first.

### ⚠ Breaking changes (consumers must update)

| Area | Before | After | Migration |
|------|--------|-------|-----------|
| **Weather alert times** | `startTimestamp` / `endTimestamp` (unix seconds, as a quoted string) | `startTime` / `endTime` (RFC3339 string, e.g. `"2026-06-26T02:33:00Z"`) | Parse RFC3339 instead of `parseInt(...)*1000`. |
| **Enum values** | lowercase / mixed strings | UPPER_SNAKE enum names | See enum table below. |
| **Fire weather location** | `weatherData[].fireWeather` (one per location, all identical) | top-level `fireWeather` on the `/weather` and `/weather/{id}` responses | Read `response.fireWeather` once instead of per location. |
| **Empty timestamps** | `""` (empty string) | `null` / omitted | Treat missing time as null. Affects `fireWeather.effective/expires`, `chainControlInfo.effectiveTime`. |
| **Incidents URL** | _new this session, no migration_ | `GET /api/v1/incidents/{area}` (area is a path param) | n/a (endpoint is brand new). |
| **Metrics URL** | `GET /api/v1/roads/metrics` (returned all-zeros) | `GET /api/v1/metrics` (returns `501 Unimplemented` until real metrics exist) | Stop relying on it; it was never real data. |
| **Client errors** | `500` for unknown road/location/area | `404` (unknown id) / `400` (bad input) | Handle 4xx as "not found / bad request", not server error. |

**Enum value changes** (JSON string values):

| Field | Before | After |
|-------|--------|-------|
| `roads[].alerts[].impact` | `"none"`,`"light"`,`"moderate"`,`"severe"` | `"IMPACT_NONE"`,`"IMPACT_LIGHT"`,`"IMPACT_MODERATE"`,`"IMPACT_SEVERE"` |
| `roads[].alerts[].duration` | `"unknown"`,`"< 1 hour"`,`"several hours"`,`"ongoing"` | `"DURATION_UNKNOWN"`,`"DURATION_UNDER_ONE_HOUR"`,`"DURATION_SEVERAL_HOURS"`,`"DURATION_ONGOING"` |
| `incidents[].status` | `"active"` | `"ACTIVE"` |
| `fireWeather.state` | `"normal"`,`"elevated"`,`"red-flag"` | `"NORMAL"`,`"ELEVATED"`,`"RED_FLAG"` |
| weather `alerts[].source` | `"NWS"`, `"OpenWeatherMap"` | `"NWS"`, `"OPENWEATHERMAP"` |
| weather `alerts[].severity` | NWS text (`"Severe"`,`"Moderate"`,`"Minor"`) | `"CRITICAL"`,`"WARNING"`,`"INFO"` (shared scale) |

(Existing road `status`, `congestionLevel`, alert `type`/`severity`/`classification`
were already UPPER_SNAKE enums and are unchanged.)

### Added

- **Region-wide incidents feed**: `GET /api/v1/incidents/{area}` (e.g.
  `/api/v1/incidents/mother-lode`) — a flat list of CHP/Caltrans dispatch
  incidents in an area, independent of the monitored roads. Each incident has
  `id`, `type`, `severity`, `location`, `locationDescription`, `description`,
  `status`, `logNumber`, `started`, `lastUpdated`, `area`.
- **Authoritative NWS weather alerts**: `/weather/alerts` now returns NWS zone
  alerts (`source: "NWS"`, with `severity` and `zones`) alongside OpenWeatherMap
  alerts (`source: "OPENWEATHERMAP"`). New `?zones=CAZ064,CAZ065` filter narrows
  the NWS alerts (OpenWeatherMap alerts are not zone-scoped and always pass).
- **Fire-weather classification**: top-level `fireWeather` on `/weather` and
  `/weather/{id}` — `state` is `NORMAL` → `ELEVATED` (Fire Weather Watch) →
  `RED_FLAG` (Red Flag Warning), only ever `RED_FLAG` when NWS confirms it.
- **Road alert id**: `roads[].alerts[].id` (CHP log / closure number) — matches
  `incidents[].id` for the same event, so per-road alerts and the region feed can
  be correlated.
- **Chain control**: `roads[].chainControlInfo` (level R1/R2/R3, location,
  direction, `effectiveTime`) from the Caltrans chain-control feed.
- **Coverage**: Hwy 49 (Angels Camp ↔ Sonora) road; weather for Sonora, Columbia,
  Twain Harte, Dorrington.
- **HTTP caching**: read endpoints now send `Cache-Control: public, max-age=60`
  and `Last-Modified`.
- **CORS**: `https://www.ersn.net`, `https://ersn.net`, `https://sierragridteam.org`
  (and `www.`) are allowlisted; browser `fetch()` now receives
  `Access-Control-Allow-Origin`.

### Changed

- Incident `description` is humanized (`"1182-Trfc Collision-No Inj"` →
  `"Traffic Collision-No Injury"`).
- Incidents are de-duplicated and geometry-only placemarks dropped, so the feed
  is one clean entry per incident.
- All timestamps across the API are RFC3339 (`google.protobuf.Timestamp`).

### Fixed

- CORS: `Access-Control-Allow-Origin` is now emitted for allowlisted origins;
  `Access-Control-Allow-Methods` is correctly restricted to `GET`; the needless
  `Access-Control-Allow-Credentials` header was removed.
- Unknown road / location / area now return `404`, and bad input `400`, instead
  of `500`.
