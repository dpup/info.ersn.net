# gRPC Services

These implement the proto services in `api/v1`. They orchestrate clients,
caching, route classification, and AI enhancement.

| File              | Responsibility |
|-------------------|----------------|
| `roads.go`        | `RoadsService`: per-road traffic, alerts, status, chain control. |
| `incidents.go`    | `RoadsService.ListIncidents`: region-wide CHP/Caltrans incident feed. |
| `weather.go`      | `WeatherService`: current conditions + combined alerts list. |
| `weather_nws.go`  | NWS zone alerts + fire-weather classification for `WeatherService`. |
| `periodic_refresh.go` | Background goroutine that warms the roads cache. |

## Caching model (read this before adding an endpoint)

Every read endpoint follows the same shape:

1. `GetWithMetadata(key, &dst)` — serve cached data; `IsStale`/`IsVeryStale`
   decide freshness.
2. On miss/stale, refresh from upstream, then `Set(key, data, ttl, kind)`.
3. On refresh failure, fall back to stale cache rather than erroring.

The cache is in-memory JSON (TTL-based), so any value must be JSON-serializable
(this is why `nws.Alert` uses exported fields). TTLs: API data ~5–15m,
AI-enhanced alerts 24h (keyed by content hash to dedupe OpenAI calls).

Roads are kept warm by `periodic_refresh.go`; weather/incidents refresh lazily on
request. Google Routes has a separate 20-minute cache (`google_routes_<id>`) to
stay within the monthly API budget — adding monitored roads increases that load.

## Adding a new endpoint

1. Add the RPC + messages to the relevant `.proto`, then `make proto`
   (see root CLAUDE.md for the toolchain — Go/protoc are not pre-installed).
2. Implement the method on the existing service struct (the gateway wiring in
   `cmd/server/main.go` is already registered per-service, so new RPCs on an
   existing service need no extra registration).
3. Request fields map automatically: fields named in the path template are path
   params (`/incidents/{area}` → `ListIncidentsRequest.area`), the rest become
   query params (`?zones=` → repeated `zones`). Convention: path params identify
   a resource (road/location/area id); query params filter a collection.
4. Add focused unit tests next to the file (construct inputs directly; don't hit
   the network).

## Region-wide incidents (`incidents.go`)

Surfaces the same Caltrans/CHP data as road alerts, but as a flat list scoped by
a configured bounding box (`roads.incidentAreas`) instead of per-route. It is
intentionally **not** AI-enhanced — region-wide volume would be too costly — so
parsing of log number / type / location / time is done structurally from the KML
description. See `internal/clients/CLAUDE.md` for the 2026 feed-format caveat.

Each incident normalizes to the same primitives the other APIs use (shared
`AlertType`/`AlertSeverity` enums, `Coordinates`, `google.protobuf.Timestamp`,
`location_description`). `normalizeIncidents` then keeps the list clean:

- **Drops geometry-only placemarks.** The lane-closure feed emits a separate
  LineString "path" placemark per closure with no description — skipped by the
  empty-description check.
- **Dedupes by `id`.** Closures are repeated across directions in `lcs2way`;
  only the first is kept.

CHP incidents carry a `started` time; lane closures are scheduled operations with
no dispatch time, so their `started` is null (expected, not a bug).

## Weather alerts & fire weather

`ListWeatherAlerts` returns authoritative **NWS** zone alerts first (source
`NWS`) followed by OpenWeatherMap alerts (source `OPENWEATHERMAP`), so clients
can prefer NWS. `?zones=CAZ064,...` filters to NWS alerts in those zones.

`fire_weather` is **region-wide** (NWS fire-weather products are issued by zone,
not point), so it lives on the response (`ListWeatherResponse` /
`GetLocationWeatherResponse`) computed once from the configured `weather.nws.zones`
— not duplicated on every location.
