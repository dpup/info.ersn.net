package nws

import "testing"

func TestClassifyFireWeather(t *testing.T) {
	redFlag := Alert{Event: eventRedFlagWarning, Headline: "RFW", Zones: []string{"CAZ064"}}
	watch := Alert{Event: eventFireWeatherWatch, Headline: "FWW", Zones: []string{"CAZ065"}}
	heat := Alert{Event: "Heat Advisory", Zones: []string{"CAZ064"}}

	tests := []struct {
		name   string
		alerts []Alert
		zones  []string
		want   string
		event  string
	}{
		{"no alerts", nil, []string{"CAZ064"}, FireWeatherNormal, ""},
		{"only heat advisory", []Alert{heat}, []string{"CAZ064"}, FireWeatherNormal, ""},
		{"red flag wins over watch", []Alert{watch, redFlag}, []string{"CAZ064", "CAZ065"}, FireWeatherRedFlag, eventRedFlagWarning},
		{"watch only", []Alert{watch}, []string{"CAZ065"}, FireWeatherElevated, eventFireWeatherWatch},
		{"red flag outside zone filtered out", []Alert{redFlag}, []string{"CAZ999"}, FireWeatherNormal, ""},
		{"no zone filter considers all", []Alert{redFlag}, nil, FireWeatherRedFlag, eventRedFlagWarning},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fw := ClassifyFireWeather(tt.alerts, tt.zones)
			if fw.State != tt.want {
				t.Errorf("state = %q, want %q", fw.State, tt.want)
			}
			if fw.SourceEvent != tt.event {
				t.Errorf("source event = %q, want %q", fw.SourceEvent, tt.event)
			}
		})
	}
}
