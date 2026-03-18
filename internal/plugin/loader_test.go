package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_RegisterBuiltin(t *testing.T) {
	r := NewRegistry()
	l := NewLoader("/tmp/test-plugins", r)

	p := &mockVideoEffect{
		mockPlugin: mockPlugin{name: "builtin-effect", version: "1.0.0", ext: ExtensionVideoEffect},
	}

	if err := l.RegisterBuiltin(p); err != nil {
		t.Fatalf("RegisterBuiltin() error = %v", err)
	}

	// Should be registered and active
	if !r.IsRegistered("builtin-effect") {
		t.Error("plugin should be registered")
	}
	if !r.IsActive("builtin-effect") {
		t.Error("plugin should be active")
	}

	// Should be resolvable as video effect
	vp := r.GetVideoEffect("builtin-effect")
	if vp == nil {
		t.Error("should resolve as VideoEffectPlugin")
	}
}

func TestLoader_LoadAll_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	r := NewRegistry()
	l := NewLoader(tmpDir, r)

	if err := l.LoadAll(); err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if r.Count() != 0 {
		t.Errorf("Count() = %d, want 0", r.Count())
	}
}

func TestLoader_LoadAll_NonexistentDir(t *testing.T) {
	r := NewRegistry()
	l := NewLoader("/tmp/nonexistent-plugin-dir-12345", r)

	// Should not error on nonexistent directory
	if err := l.LoadAll(); err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
}

func TestLoader_LoadAll_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a plugin directory with invalid manifest
	pluginDir := filepath.Join(tmpDir, "bad-plugin")
	os.MkdirAll(pluginDir, 0755)
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte("invalid: yaml: ["), 0644)

	r := NewRegistry()
	l := NewLoader(tmpDir, r)

	// Should not error - invalid plugins are skipped with warning
	if err := l.LoadAll(); err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if r.Count() != 0 {
		t.Errorf("Count() = %d, want 0 (invalid plugin should be skipped)", r.Count())
	}
}

func TestLoader_PluginsDir(t *testing.T) {
	r := NewRegistry()
	l := NewLoader("/opt/plugins", r)

	if l.PluginsDir() != "/opt/plugins" {
		t.Errorf("PluginsDir() = %q, want %q", l.PluginsDir(), "/opt/plugins")
	}
}
