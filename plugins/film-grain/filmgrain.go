// Package filmgrain implements a cinematic film grain effect plugin.
//
// This is a built-in plugin that demonstrates the plugin system.
// It generates an FFmpeg noise filter that simulates analog film grain.
//
// FFmpeg filter chain:
//   noise=alls={strength}:allf=t+u
//
// Parameters:
//   - intensity: float (0.0-1.0) - maps to noise strength (0-30)
//   - size: float (0.1-5.0) - unused in FFmpeg noise filter but reserved for future use
package filmgrain

import (
	"fmt"

	"github.com/prmichaelsen/cloudcut-media-server/internal/plugin"
)

const (
	pluginName    = "film-grain"
	pluginVersion = "1.0.0"
)

// FilmGrainPlugin implements the VideoEffectPlugin interface.
type FilmGrainPlugin struct{}

// Ensure interface compliance at compile time.
var _ plugin.VideoEffectPlugin = (*FilmGrainPlugin)(nil)

func (p *FilmGrainPlugin) Name() string {
	return pluginName
}

func (p *FilmGrainPlugin) Version() string {
	return pluginVersion
}

func (p *FilmGrainPlugin) ExtensionPoint() plugin.ExtensionPoint {
	return plugin.ExtensionVideoEffect
}

func (p *FilmGrainPlugin) Init() error {
	return nil
}

func (p *FilmGrainPlugin) Destroy() error {
	return nil
}

// BuildFFmpegFilter returns the FFmpeg noise filter string for film grain.
// Maps intensity (0.0-1.0) to FFmpeg noise strength (0-30).
func (p *FilmGrainPlugin) BuildFFmpegFilter(params map[string]interface{}) (string, error) {
	intensity := 0.5 // default
	if v, ok := params["intensity"]; ok {
		if f, ok := toFloat64(v); ok {
			intensity = f
		}
	}

	// Clamp intensity to 0.0-1.0
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}

	// Map intensity to FFmpeg noise strength (0-30)
	strength := int(intensity * 30)
	if strength == 0 && intensity > 0 {
		strength = 1 // Minimum visible grain
	}

	// noise filter with temporal (t) and uniform (u) flags
	// alls = all streams strength, allf = all streams flags
	return fmt.Sprintf("noise=alls=%d:allf=t+u", strength), nil
}

// ValidateParams checks if the given parameters are valid.
func (p *FilmGrainPlugin) ValidateParams(params map[string]interface{}) error {
	if v, ok := params["intensity"]; ok {
		f, ok := toFloat64(v)
		if !ok {
			return fmt.Errorf("intensity must be a number")
		}
		if f < 0 || f > 1 {
			return fmt.Errorf("intensity must be between 0.0 and 1.0")
		}
	}

	if v, ok := params["size"]; ok {
		f, ok := toFloat64(v)
		if !ok {
			return fmt.Errorf("size must be a number")
		}
		if f < 0.1 || f > 5.0 {
			return fmt.Errorf("size must be between 0.1 and 5.0")
		}
	}

	return nil
}

// DefaultParams returns the default parameters for the film grain effect.
func (p *FilmGrainPlugin) DefaultParams() map[string]interface{} {
	return map[string]interface{}{
		"intensity": 0.5,
		"size":      1.0,
	}
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
