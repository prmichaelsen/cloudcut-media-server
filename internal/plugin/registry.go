package plugin

import (
	"fmt"
	"log"
	"sync"
)

// Registry manages plugin discovery, loading, and resolution.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin // keyed by plugin name
	active  map[string]bool   // tracks which plugins are active
}

// NewRegistry creates a new empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
		active:  make(map[string]bool),
	}
}

// Register adds a plugin to the registry.
// If a plugin with the same name is already registered, it returns an error.
func (r *Registry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %q already registered", name)
	}

	r.plugins[name] = p
	r.active[name] = false

	log.Printf("plugin registered: %s v%s (%s)", name, p.Version(), p.ExtensionPoint())
	return nil
}

// Activate initializes and activates a plugin.
func (r *Registry) Activate(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}

	if r.active[name] {
		return nil // Already active
	}

	if err := p.Init(); err != nil {
		return fmt.Errorf("plugin %q init failed: %w", name, err)
	}

	r.active[name] = true
	log.Printf("plugin activated: %s", name)
	return nil
}

// Deactivate destroys and deactivates a plugin.
func (r *Registry) Deactivate(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}

	if !r.active[name] {
		return nil // Already inactive
	}

	if err := p.Destroy(); err != nil {
		return fmt.Errorf("plugin %q destroy failed: %w", name, err)
	}

	r.active[name] = false
	log.Printf("plugin deactivated: %s", name)
	return nil
}

// Unregister removes a plugin from the registry.
// If the plugin is active, it is deactivated first.
func (r *Registry) Unregister(name string) error {
	// Deactivate first if active
	r.mu.RLock()
	isActive := r.active[name]
	r.mu.RUnlock()

	if isActive {
		if err := r.Deactivate(name); err != nil {
			return fmt.Errorf("deactivate before unregister: %w", err)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.plugins, name)
	delete(r.active, name)

	log.Printf("plugin unregistered: %s", name)
	return nil
}

// Get returns a plugin by name. Returns nil if not found.
func (r *Registry) Get(name string) Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.plugins[name]
}

// GetVideoEffect returns a plugin as a VideoEffectPlugin, or nil if not found or wrong type.
func (r *Registry) GetVideoEffect(name string) VideoEffectPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.plugins[name]
	if !ok {
		return nil
	}

	vp, ok := p.(VideoEffectPlugin)
	if !ok {
		return nil
	}

	// Only return active plugins
	if !r.active[name] {
		return nil
	}

	return vp
}

// GetAudioEffect returns a plugin as an AudioEffectPlugin, or nil if not found or wrong type.
func (r *Registry) GetAudioEffect(name string) AudioEffectPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.plugins[name]
	if !ok {
		return nil
	}

	ap, ok := p.(AudioEffectPlugin)
	if !ok {
		return nil
	}

	if !r.active[name] {
		return nil
	}

	return ap
}

// IsRegistered checks if a plugin with the given name is registered.
func (r *Registry) IsRegistered(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.plugins[name]
	return ok
}

// IsActive checks if a plugin is active.
func (r *Registry) IsActive(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.active[name]
}

// List returns info about all registered plugins.
func (r *Registry) List() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]PluginInfo, 0, len(r.plugins))
	for name, p := range r.plugins {
		infos = append(infos, PluginInfo{
			Name:           name,
			Version:        p.Version(),
			ExtensionPoint: p.ExtensionPoint(),
			Active:         r.active[name],
			Source:         "builtin",
		})
	}
	return infos
}

// ListByExtension returns all plugins for a given extension point.
func (r *Registry) ListByExtension(ext ExtensionPoint) []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var infos []PluginInfo
	for name, p := range r.plugins {
		if p.ExtensionPoint() == ext {
			infos = append(infos, PluginInfo{
				Name:           name,
				Version:        p.Version(),
				ExtensionPoint: ext,
				Active:         r.active[name],
				Source:         "builtin",
			})
		}
	}
	return infos
}

// Count returns the number of registered plugins.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.plugins)
}

// ActivateAll activates all registered plugins. Returns the first error encountered.
func (r *Registry) ActivateAll() error {
	r.mu.RLock()
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	r.mu.RUnlock()

	for _, name := range names {
		if err := r.Activate(name); err != nil {
			return err
		}
	}
	return nil
}

// DeactivateAll deactivates all active plugins.
func (r *Registry) DeactivateAll() {
	r.mu.RLock()
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		if r.active[name] {
			names = append(names, name)
		}
	}
	r.mu.RUnlock()

	for _, name := range names {
		if err := r.Deactivate(name); err != nil {
			log.Printf("error deactivating plugin %s: %v", name, err)
		}
	}
}
