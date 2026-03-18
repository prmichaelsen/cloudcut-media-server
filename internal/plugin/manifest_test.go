package plugin

import (
	"testing"
)

func TestParseManifestBytes_Valid(t *testing.T) {
	yaml := `
name: film-grain
version: 1.0.0
description: Adds cinematic film grain effect
author: CloudCut
license: MIT
extension_point: effects.video
plugin_type: go
entry_point: film-grain.so
params:
  - name: intensity
    type: float
    description: Grain intensity (0.0-1.0)
    default: 0.5
    min: 0.0
    max: 1.0
    required: false
  - name: size
    type: float
    description: Grain size multiplier
    default: 1.0
    min: 0.1
    max: 5.0
    required: false
`

	m, err := ParseManifestBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseManifestBytes() error = %v", err)
	}

	if m.Name != "film-grain" {
		t.Errorf("Name = %q, want %q", m.Name, "film-grain")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.0.0")
	}
	if m.ExtensionPoint != ExtensionVideoEffect {
		t.Errorf("ExtensionPoint = %q, want %q", m.ExtensionPoint, ExtensionVideoEffect)
	}
	if m.PluginType != "go" {
		t.Errorf("PluginType = %q, want %q", m.PluginType, "go")
	}
	if len(m.Params) != 2 {
		t.Errorf("len(Params) = %d, want 2", len(m.Params))
	}
	if m.Params[0].Name != "intensity" {
		t.Errorf("Params[0].Name = %q, want %q", m.Params[0].Name, "intensity")
	}
}

func TestParseManifestBytes_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "missing name",
			yaml: `version: 1.0.0
extension_point: effects.video
plugin_type: go
entry_point: test.so`,
		},
		{
			name: "missing version",
			yaml: `name: test
extension_point: effects.video
plugin_type: go
entry_point: test.so`,
		},
		{
			name: "missing extension_point",
			yaml: `name: test
version: 1.0.0
plugin_type: go
entry_point: test.so`,
		},
		{
			name: "invalid extension_point",
			yaml: `name: test
version: 1.0.0
extension_point: invalid
plugin_type: go
entry_point: test.so`,
		},
		{
			name: "missing plugin_type",
			yaml: `name: test
version: 1.0.0
extension_point: effects.video
entry_point: test.so`,
		},
		{
			name: "unsupported plugin_type",
			yaml: `name: test
version: 1.0.0
extension_point: effects.video
plugin_type: wasm
entry_point: test.wasm`,
		},
		{
			name: "missing entry_point",
			yaml: `name: test
version: 1.0.0
extension_point: effects.video
plugin_type: go`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseManifestBytes([]byte(tt.yaml))
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestParseManifestBytes_InvalidParamType(t *testing.T) {
	yaml := `
name: test
version: 1.0.0
extension_point: effects.video
plugin_type: go
entry_point: test.so
params:
  - name: value
    type: complex
    description: bad type
`
	_, err := ParseManifestBytes([]byte(yaml))
	if err == nil {
		t.Error("expected error for invalid param type")
	}
}

func TestValidateParamsAgainstManifest(t *testing.T) {
	min := 0.0
	max := 1.0
	m := &Manifest{
		Params: []ParamSpec{
			{Name: "intensity", Type: "float", Min: &min, Max: &max, Required: true},
			{Name: "label", Type: "string", Required: false},
		},
	}

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid params",
			params:  map[string]interface{}{"intensity": 0.5},
			wantErr: false,
		},
		{
			name:    "missing required",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "wrong type",
			params:  map[string]interface{}{"intensity": "not a number"},
			wantErr: true,
		},
		{
			name:    "below min",
			params:  map[string]interface{}{"intensity": -0.5},
			wantErr: true,
		},
		{
			name:    "above max",
			params:  map[string]interface{}{"intensity": 1.5},
			wantErr: true,
		},
		{
			name:    "optional string valid",
			params:  map[string]interface{}{"intensity": 0.5, "label": "test"},
			wantErr: false,
		},
		{
			name:    "optional string wrong type",
			params:  map[string]interface{}{"intensity": 0.5, "label": 123},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParamsAgainstManifest(m, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
