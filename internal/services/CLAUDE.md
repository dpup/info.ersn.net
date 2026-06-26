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
3. Query params on a `GET` map automatically from request fields (e.g.
   `?area=` → `ListIncidentsRequest.area`, `?zones=` → repeated `zones`).
4. Add focused unit tests next to the file (construct inputs directly; don't hit
   the network).

## Region-wide incidents (`incidents.go`)

Surfaces the same Caltrans/CHP data as road alerts, but as a flat list scoped by
a configured bounding box (`roads.incidentAreas`) instead of per-route. It is
intentionally **not** AI-enhanced — region-wide volume would be too costly — so
parsing of log number / type / location / time is done structurally from the KML
description. See `internal/clients/CLAUDE.md` for the 2026 feed-format caveat.

## Weather alerts & fire weather

`ListWeatherAlerts` returns authoritative **NWS** zone alerts first (source
`"NWS"`) followed by OpenWeatherMap per-location alerts (source
`"OpenWeatherMap"`), so clients can prefer NWS. `?zones=CAZ064,...` filters to NWS
alerts in those zones. Per-location `fire_weather` is derived from the same NWS
alert set, defaulting to the region zones unless a location sets `nwsZones`.
