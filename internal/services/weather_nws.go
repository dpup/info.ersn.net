package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/dpup/prefab/logging"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/dpup/info.ersn.net/server/api/v1"
	"github.com/dpup/info.ersn.net/server/internal/clients/nws"
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
			Source:      api.AlertSource_NWS,
			Severity:    mapNWSSeverity(a.Severity),
			Zones:       a.Zones,
		}
		if !a.Effective.IsZero() {
			wa.StartTime = timestamppb.New(a.Effective)
		}
		if !a.Expires.IsZero() {
			wa.EndTime = timestamppb.New(a.Expires)
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

// computeRegionFireWeather classifies fire-weather risk for the whole service
// area from the shared NWS alert list. Fire-weather products are regional, so a
// single classification applies to every monitored location.
func (s *WeatherService) computeRegionFireWeather(ctx context.Context) *api.FireWeather {
	fw := nws.ClassifyFireWeather(s.getNWSAlerts(ctx), s.config.Weather.NWS.Zones)
	out := &api.FireWeather{
		State:       mapFireWeatherState(fw.State),
		SourceEvent: fw.SourceEvent,
		Headline:    fw.Headline,
		SenderName:  fw.SenderName,
		Zones:       fw.Zones,
	}
	if !fw.Effective.IsZero() {
		out.Effective = timestamppb.New(fw.Effective)
	}
	if !fw.Expires.IsZero() {
		out.Expires = timestamppb.New(fw.Expires)
	}
	return out
}

// mapNWSSeverity maps NWS severity terms onto the shared AlertSeverity scale.
func mapNWSSeverity(s string) api.AlertSeverity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "extreme", "severe":
		return api.AlertSeverity_CRITICAL
	case "moderate":
		return api.AlertSeverity_WARNING
	case "minor":
		return api.AlertSeverity_INFO
	default:
		return api.AlertSeverity_ALERT_SEVERITY_UNSPECIFIED
	}
}

// mapFireWeatherState maps the nws package's string state to the proto enum.
func mapFireWeatherState(s string) api.FireWeatherState {
	switch s {
	case nws.FireWeatherNormal:
		return api.FireWeatherState_NORMAL
	case nws.FireWeatherElevated:
		return api.FireWeatherState_ELEVATED
	case nws.FireWeatherRedFlag:
		return api.FireWeatherState_RED_FLAG
	default:
		return api.FireWeatherState_FIRE_WEATHER_STATE_UNSPECIFIED
	}
}
