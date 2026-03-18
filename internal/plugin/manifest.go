package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Manifest describes a plugin's metadata and configuration.
type Manifest struct {
	Name           string         `yaml:"name" json:"name"`
	Version        string         `yaml:"version" json:"version"`
	Description    string         `yaml:"description" json:"description"`
	Author         string         `yaml:"author" json:"author"`
	License        string         `yaml:"license" json:"license"`
	ExtensionPoint ExtensionPoint `yaml:"extension_point" json:"extensionPoint"`
	PluginType     string         `yaml:"plugin_type" json:"pluginType"` // "go" for now
	EntryPoint     string         `yaml:"entry_point" json:"entryPoint"` // path to .so file
	Params         []ParamSpec    `yaml:"params" json:"params"`
}

// ParamSpec describes a plugin parameter.
type ParamSpec struct {
	Name        string      `yaml:"name" json:"name"`
	Type        string      `yaml:"type" json:"type"` // "float", "int", "string", "bool"
	Description string      `yaml:"description" json:"description"`
	Default     interface{} `yaml:"default" json:"default"`
	Min         *float64    `yaml:"min,omitempty" json:"min,omitempty"`
	Max         *float64    `yaml:"max,omitempty" json:"max,omitempty"`
	Required    bool        `yaml:"required" json:"required"`
}

// ParseManifest reads and parses a plugin.yaml manifest file.
func ParseManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}

	return ParseManifestBytes(data)
}

// ParseManifestBytes parses manifest YAML bytes.
func ParseManifestBytes(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if err := validateManifest(&m); err != nil {
		return nil, err
	}

	return &m, nil
}

// DiscoverManifests searches a directory for plugin.yaml files.
func DiscoverManifests(pluginsDir string) ([]*Manifest, error) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No plugins directory is fine
		}
		return nil, fmt.Errorf("read plugins directory: %w", err)
	}

	var manifests []*Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(pluginsDir, entry.Name(), "plugin.yaml")
		m, err := ParseManifest(manifestPath)
		if err != nil {
			// Log but don't fail on individual plugin errors
			fmt.Fprintf(os.Stderr, "warning: skipping plugin %s: %v\n", entry.Name(), err)
			continue
		}

		manifests = append(manifests, m)
	}

	return manifests, nil
}

// validateManifest checks that a manifest has all required fields.
func validateManifest(m *Manifest) error {
	if m.Name == "" {
		return fmt.Errorf("manifest: name is required")
	}

	if m.Version == "" {
		return fmt.Errorf("manifest: version is required")
	}

	if m.ExtensionPoint == "" {
		return fmt.Errorf("manifest: extension_point is required")
	}

	validExtensions := map[ExtensionPoint]bool{
		ExtensionVideoEffect:  true,
		ExtensionAudioEffect:  true,
		ExtensionExportFormat: true,
	}
	if !validExtensions[m.ExtensionPoint] {
		return fmt.Errorf("manifest: invalid extension_point %q", m.ExtensionPoint)
	}

	if m.PluginType == "" {
		return fmt.Errorf("manifest: plugin_type is required")
	}

	validTypes := map[string]bool{"go": true}
	if !validTypes[m.PluginType] {
		return fmt.Errorf("manifest: unsupported plugin_type %q (supported: go)", m.PluginType)
	}

	if m.EntryPoint == "" {
		return fmt.Errorf("manifest: entry_point is required")
	}

	// Validate param specs
	for i, p := range m.Params {
		if p.Name == "" {
			return fmt.Errorf("manifest: params[%d].name is required", i)
		}
		validParamTypes := map[string]bool{"float": true, "int": true, "string": true, "bool": true}
		if !validParamTypes[p.Type] {
			return fmt.Errorf("manifest: params[%d].type %q is invalid (supported: float, int, string, bool)", i, p.Type)
		}
	}

	return nil
}

// ValidateParamsAgainstManifest checks user-supplied params against the manifest schema.
func ValidateParamsAgainstManifest(m *Manifest, params map[string]interface{}) error {
	// Check required params
	for _, spec := range m.Params {
		val, exists := params[spec.Name]

		if spec.Required && !exists {
			return fmt.Errorf("required parameter %q is missing", spec.Name)
		}

		if !exists {
			continue
		}

		// Type checking
		switch spec.Type {
		case "float":
			v, ok := toFloat64(val)
			if !ok {
				return fmt.Errorf("parameter %q must be a number", spec.Name)
			}
			if spec.Min != nil && v < *spec.Min {
				return fmt.Errorf("parameter %q must be >= %.2f", spec.Name, *spec.Min)
			}
			if spec.Max != nil && v > *spec.Max {
				return fmt.Errorf("parameter %q must be <= %.2f", spec.Name, *spec.Max)
			}
		case "int":
			_, ok := toFloat64(val)
			if !ok {
				return fmt.Errorf("parameter %q must be a number", spec.Name)
			}
		case "string":
			if _, ok := val.(string); !ok {
				return fmt.Errorf("parameter %q must be a string", spec.Name)
			}
		case "bool":
			if _, ok := val.(bool); !ok {
				return fmt.Errorf("parameter %q must be a boolean", spec.Name)
			}
		}
	}

	return nil
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
