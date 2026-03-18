package plugin

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	goplugin "plugin"
)

// Loader discovers and loads Go plugins from a directory.
type Loader struct {
	pluginsDir string
	registry   *Registry
}

// NewLoader creates a new plugin loader.
func NewLoader(pluginsDir string, registry *Registry) *Loader {
	return &Loader{
		pluginsDir: pluginsDir,
		registry:   registry,
	}
}

// LoadAll discovers and loads all plugins from the plugins directory.
// Each plugin must have a plugin.yaml manifest and a corresponding .so file.
func (l *Loader) LoadAll() error {
	manifests, err := DiscoverManifests(l.pluginsDir)
	if err != nil {
		return fmt.Errorf("discover manifests: %w", err)
	}

	if len(manifests) == 0 {
		log.Printf("no plugins found in %s", l.pluginsDir)
		return nil
	}

	for _, m := range manifests {
		if err := l.loadPlugin(m); err != nil {
			log.Printf("warning: failed to load plugin %s: %v", m.Name, err)
			continue
		}
	}

	return nil
}

// LoadPlugin loads a single plugin from its manifest.
func (l *Loader) loadPlugin(m *Manifest) error {
	switch m.PluginType {
	case "go":
		return l.loadGoPlugin(m)
	default:
		return fmt.Errorf("unsupported plugin type: %s", m.PluginType)
	}
}

// loadGoPlugin loads a Go plugin (.so) using the plugin package.
func (l *Loader) loadGoPlugin(m *Manifest) error {
	// Resolve the .so path relative to the plugin directory
	soPath := filepath.Join(l.pluginsDir, m.Name, m.EntryPoint)

	// Check if the .so file exists
	if _, err := os.Stat(soPath); err != nil {
		return fmt.Errorf("plugin binary not found at %s: %w", soPath, err)
	}

	// Open the .so file
	p, err := goplugin.Open(soPath)
	if err != nil {
		return fmt.Errorf("open plugin %s: %w", soPath, err)
	}

	// Look up the NewPlugin symbol
	sym, err := p.Lookup("NewPlugin")
	if err != nil {
		return fmt.Errorf("plugin %s missing NewPlugin symbol: %w", m.Name, err)
	}

	// Assert the symbol is a function returning Plugin
	newPlugin, ok := sym.(func() Plugin)
	if !ok {
		// Try VideoEffectPlugin factory
		newVideoPlugin, ok := sym.(func() VideoEffectPlugin)
		if !ok {
			return fmt.Errorf("plugin %s: NewPlugin has wrong signature (expected func() Plugin or func() VideoEffectPlugin)", m.Name)
		}

		plugin := newVideoPlugin()
		if err := l.registry.Register(plugin); err != nil {
			return fmt.Errorf("register plugin %s: %w", m.Name, err)
		}

		log.Printf("loaded Go plugin: %s v%s from %s", m.Name, m.Version, soPath)
		return nil
	}

	plugin := newPlugin()
	if err := l.registry.Register(plugin); err != nil {
		return fmt.Errorf("register plugin %s: %w", m.Name, err)
	}

	log.Printf("loaded Go plugin: %s v%s from %s", m.Name, m.Version, soPath)
	return nil
}

// RegisterBuiltin registers a built-in plugin directly (no .so loading needed).
// This is useful for plugins compiled into the server binary.
func (l *Loader) RegisterBuiltin(p Plugin) error {
	if err := l.registry.Register(p); err != nil {
		return err
	}

	if err := l.registry.Activate(p.Name()); err != nil {
		return fmt.Errorf("activate builtin plugin %s: %w", p.Name(), err)
	}

	return nil
}

// PluginsDir returns the configured plugins directory.
func (l *Loader) PluginsDir() string {
	return l.pluginsDir
}
