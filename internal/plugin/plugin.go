package plugin

// ExtensionPoint identifies what kind of plugin this is.
type ExtensionPoint string

const (
	ExtensionVideoEffect ExtensionPoint = "effects.video"
	ExtensionAudioEffect ExtensionPoint = "effects.audio"
	ExtensionExportFormat ExtensionPoint = "export.formats"
)

// Plugin is the interface that all plugins must implement.
type Plugin interface {
	// Name returns the unique plugin identifier (e.g., "film-grain").
	Name() string

	// Version returns the plugin version string.
	Version() string

	// ExtensionPoint returns what the plugin extends.
	ExtensionPoint() ExtensionPoint

	// Init is called when the plugin is loaded.
	Init() error

	// Destroy is called when the plugin is unloaded.
	Destroy() error
}

// VideoEffectPlugin extends Plugin with video filter capabilities.
type VideoEffectPlugin interface {
	Plugin

	// BuildFFmpegFilter returns the FFmpeg filter string for this effect.
	// params contains the user-supplied parameters from the EDL filter.
	BuildFFmpegFilter(params map[string]interface{}) (string, error)

	// ValidateParams checks if the given parameters are valid for this effect.
	ValidateParams(params map[string]interface{}) error

	// DefaultParams returns the default parameters for this effect.
	DefaultParams() map[string]interface{}
}

// AudioEffectPlugin extends Plugin with audio filter capabilities.
type AudioEffectPlugin interface {
	Plugin

	// BuildFFmpegFilter returns the FFmpeg audio filter string for this effect.
	BuildFFmpegFilter(params map[string]interface{}) (string, error)

	// ValidateParams checks if the given parameters are valid for this effect.
	ValidateParams(params map[string]interface{}) error

	// DefaultParams returns the default parameters for this effect.
	DefaultParams() map[string]interface{}
}

// PluginInfo contains metadata about a registered plugin.
type PluginInfo struct {
	Name           string         `json:"name"`
	Version        string         `json:"version"`
	ExtensionPoint ExtensionPoint `json:"extensionPoint"`
	Active         bool           `json:"active"`
	Source         string         `json:"source"` // "builtin", "go-plugin", "manifest"
}
