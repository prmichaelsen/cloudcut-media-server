package edl

import (
	"fmt"
	"testing"
)

type mockFilterChecker struct {
	knownFilters map[string]bool
}

func (m *mockFilterChecker) IsKnownFilter(filterType string) bool {
	return m.knownFilters[filterType]
}

func (m *mockFilterChecker) ValidateFilterParams(filterType string, params map[string]interface{}) error {
	if filterType == "film-grain" {
		if intensity, ok := params["intensity"]; ok {
			v, ok := intensity.(float64)
			if !ok || v < 0 || v > 1 {
				return fmt.Errorf("intensity must be between 0 and 1")
			}
		}
	}
	return nil
}

func TestValidateWithPlugins_BuiltinFilter(t *testing.T) {
	edl := &EDL{
		Version:   "1.0",
		ProjectID: "test",
		Timeline: Timeline{
			Duration: 10,
			Tracks: []Track{
				{
					ID:   "v1",
					Type: "video",
					Clips: []Clip{
						{
							ID: "c1", MediaID: "m1",
							StartTime: 0, Duration: 5, InPoint: 0, OutPoint: 5,
							Filters: []Filter{
								{Type: "brightness", Params: map[string]interface{}{"value": 0.5}},
							},
						},
					},
				},
			},
		},
		Output: Output{Format: "mp4", Quality: "high"},
	}

	checker := &mockFilterChecker{knownFilters: map[string]bool{}}
	errs := ValidateWithPlugins(edl, func(id string) bool { return true }, checker)

	if len(errs) > 0 {
		t.Errorf("expected no errors for built-in filter, got: %v", errs)
	}
}

func TestValidateWithPlugins_KnownPlugin(t *testing.T) {
	edl := &EDL{
		Version:   "1.0",
		ProjectID: "test",
		Timeline: Timeline{
			Duration: 10,
			Tracks: []Track{
				{
					ID:   "v1",
					Type: "video",
					Clips: []Clip{
						{
							ID: "c1", MediaID: "m1",
							StartTime: 0, Duration: 5, InPoint: 0, OutPoint: 5,
							Filters: []Filter{
								{Type: "film-grain", Params: map[string]interface{}{"intensity": 0.5}},
							},
						},
					},
				},
			},
		},
		Output: Output{Format: "mp4", Quality: "high"},
	}

	checker := &mockFilterChecker{knownFilters: map[string]bool{"film-grain": true}}
	errs := ValidateWithPlugins(edl, func(id string) bool { return true }, checker)

	if len(errs) > 0 {
		t.Errorf("expected no errors for known plugin filter, got: %v", errs)
	}
}

func TestValidateWithPlugins_UnknownFilter(t *testing.T) {
	edl := &EDL{
		Version:   "1.0",
		ProjectID: "test",
		Timeline: Timeline{
			Duration: 10,
			Tracks: []Track{
				{
					ID:   "v1",
					Type: "video",
					Clips: []Clip{
						{
							ID: "c1", MediaID: "m1",
							StartTime: 0, Duration: 5, InPoint: 0, OutPoint: 5,
							Filters: []Filter{
								{Type: "unknown-effect", Params: map[string]interface{}{}},
							},
						},
					},
				},
			},
		},
		Output: Output{Format: "mp4", Quality: "high"},
	}

	checker := &mockFilterChecker{knownFilters: map[string]bool{}}
	errs := ValidateWithPlugins(edl, func(id string) bool { return true }, checker)

	if len(errs) == 0 {
		t.Error("expected error for unknown filter type")
	}
}

func TestValidateWithPlugins_InvalidParams(t *testing.T) {
	edl := &EDL{
		Version:   "1.0",
		ProjectID: "test",
		Timeline: Timeline{
			Duration: 10,
			Tracks: []Track{
				{
					ID:   "v1",
					Type: "video",
					Clips: []Clip{
						{
							ID: "c1", MediaID: "m1",
							StartTime: 0, Duration: 5, InPoint: 0, OutPoint: 5,
							Filters: []Filter{
								{Type: "film-grain", Params: map[string]interface{}{"intensity": 5.0}},
							},
						},
					},
				},
			},
		},
		Output: Output{Format: "mp4", Quality: "high"},
	}

	checker := &mockFilterChecker{knownFilters: map[string]bool{"film-grain": true}}
	errs := ValidateWithPlugins(edl, func(id string) bool { return true }, checker)

	if len(errs) == 0 {
		t.Error("expected error for invalid plugin params")
	}
}
