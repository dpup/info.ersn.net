package services

import (
	"context"
	"fmt"

	"github.com/dpup/prefab/logging"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/clients/nws"
	"github.com/dpup/info.ersn.net/server/internal/config"
)

// getNWSAlerts returns the active NWS alerts for the configured service-area
// zones, caching the raw alert list so it is fetched at most once per weather
// refresh and shared between zone-alert listing and fire-weather classification.
func (s *WeatherService) getNWSAlerts(ctx context.Context) []nws.Alert {
	if s.nwsClient == nil || len(s.config.Weather.NWS.Zones) == 0 {
		return nil
	}

	cacheKey := "nws:alerts"
	var cached []nws.Alert
	if found, _ := s.cache.Get(cacheKey, &cached); found && !s.cache.IsStale(cacheKey) {
		return cached
	}

	alerts, err := s.nwsClient.GetActiveZoneAlerts(ctx, s.config.Weather.NWS.Zones)
	if err != nil {
		logging.Errorw(ctx, "Failed to fetch NWS zone alerts", "error", err)
		// Fall back to stale cache rather than dropping alerts on a transient error.
		if cached != nil {
			return cached
		}
		return nil
	}

	if err := s.cache.Set(cacheKey, alerts, s.config.Weather.RefreshInterval, "nws_alerts"); err != nil {
		logging.Errorw(ctx, "Failed to cache NWS alerts", "error", err)
	}
	logging.Infow(ctx, "Fetched NWS zone alerts", "zones", s.config.Weather.NWS.Zones, "count", len(alerts))
	return alerts
}

// nwsAlertsToProto converts NWS alerts into API WeatherAlerts tagged with the
// "NWS" source. NWS provides authoritative headline/description text, so these
// are surfaced as-is rather than AI-enhanced (keeps the official wording and
// avoids per-alert OpenAI cost).
func nwsAlertsToProto(alerts []nws.Alert) []*api.WeatherAlert {
	var out []*api.WeatherAlert
	for _, a := range alerts {
		wa := &api.WeatherAlert{
			Id:          nwsAlertID(a),
			SenderName:  a.SenderName,
			Event:       a.Event,
			Description: a.Description,
			Headline:    a.Headline,
			Summary:     a.Headline,
			Details:     a.Description,
			Source:      "NWS",
			Severity:    a.Severity,
			Zones:       a.Zones,
		}
		if !a.Effective.IsZero() {
			wa.StartTimestamp = a.Effective.Unix()
		}
		if !a.Expires.IsZero() {
			wa.EndTimestamp = a.Expires.Unix()
		}
		out = append(out, wa)
	}
	return out
}

func nwsAlertID(a nws.Alert) string {
	if a.ID != "" {
		return a.ID
	}
	return fmt.Sprintf("nws_%s_%d", a.Event, a.Effective.Unix())
}

// computeFireWeather classifies fire-weather risk for a location from the shared
// NWS alert list. It uses the location's NWSZones override when set, otherwise
// the region-wide configured zones.
func (s *WeatherService) computeFireWeather(location config.WeatherLocation, alerts []nws.Alert) *api.FireWeather {
	zones := location.NWSZones
	if len(zones) == 0 {
		zones = s.config.Weather.NWS.Zones
	}

	fw := nws.ClassifyFireWeather(alerts, zones)
	return &api.FireWeather{
		State:       fw.State,
		SourceEvent: fw.SourceEvent,
		Headline:    fw.Headline,
		SenderName:  fw.SenderName,
		Effective:   fw.Effective,
		Expires:     fw.Expires,
		Zones:       fw.Zones,
	}
}
