package plugin

import (
	"testing"
)

// mockPlugin implements Plugin for testing.
type mockPlugin struct {
	name       string
	version    string
	ext        ExtensionPoint
	initCalled bool
	destroyCalled bool
	initErr    error
}

func (m *mockPlugin) Name() string           { return m.name }
func (m *mockPlugin) Version() string        { return m.version }
func (m *mockPlugin) ExtensionPoint() ExtensionPoint { return m.ext }
func (m *mockPlugin) Init() error            { m.initCalled = true; return m.initErr }
func (m *mockPlugin) Destroy() error         { m.destroyCalled = true; return nil }

// mockVideoEffect implements VideoEffectPlugin for testing.
type mockVideoEffect struct {
	mockPlugin
}

func (m *mockVideoEffect) BuildFFmpegFilter(params map[string]interface{}) (string, error) {
	return "eq=brightness=0.5", nil
}

func (m *mockVideoEffect) ValidateParams(params map[string]interface{}) error {
	return nil
}

func (m *mockVideoEffect) DefaultParams() map[string]interface{} {
	return map[string]interface{}{"value": 0.5}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()

	p := &mockPlugin{name: "test-effect", version: "1.0.0", ext: ExtensionVideoEffect}

	if err := r.Register(p); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Should find the plugin
	got := r.Get("test-effect")
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.Name() != "test-effect" {
		t.Errorf("Name() = %q, want %q", got.Name(), "test-effect")
	}

	// Should not find unknown plugin
	if r.Get("unknown") != nil {
		t.Error("Get(unknown) should return nil")
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewRegistry()

	p1 := &mockPlugin{name: "dup", version: "1.0.0", ext: ExtensionVideoEffect}
	p2 := &mockPlugin{name: "dup", version: "2.0.0", ext: ExtensionVideoEffect}

	if err := r.Register(p1); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}

	if err := r.Register(p2); err == nil {
		t.Error("second Register() should return error for duplicate name")
	}
}

func TestRegistry_ActivateDeactivate(t *testing.T) {
	r := NewRegistry()

	p := &mockPlugin{name: "lifecycle", version: "1.0.0", ext: ExtensionVideoEffect}
	r.Register(p)

	// Should not be active initially
	if r.IsActive("lifecycle") {
		t.Error("should not be active before Activate()")
	}

	// Activate
	if err := r.Activate("lifecycle"); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	if !p.initCalled {
		t.Error("Init() should have been called")
	}
	if !r.IsActive("lifecycle") {
		t.Error("should be active after Activate()")
	}

	// Activate again (should be no-op)
	if err := r.Activate("lifecycle"); err != nil {
		t.Fatalf("second Activate() error = %v", err)
	}

	// Deactivate
	if err := r.Deactivate("lifecycle"); err != nil {
		t.Fatalf("Deactivate() error = %v", err)
	}

	if !p.destroyCalled {
		t.Error("Destroy() should have been called")
	}
	if r.IsActive("lifecycle") {
		t.Error("should not be active after Deactivate()")
	}
}

func TestRegistry_GetVideoEffect(t *testing.T) {
	r := NewRegistry()

	vp := &mockVideoEffect{
		mockPlugin: mockPlugin{name: "brightness", version: "1.0.0", ext: ExtensionVideoEffect},
	}
	r.Register(vp)
	r.Activate("brightness")

	// Should resolve as video effect
	got := r.GetVideoEffect("brightness")
	if got == nil {
		t.Fatal("GetVideoEffect() returned nil")
	}

	filter, err := got.BuildFFmpegFilter(nil)
	if err != nil {
		t.Fatalf("BuildFFmpegFilter() error = %v", err)
	}
	if filter != "eq=brightness=0.5" {
		t.Errorf("filter = %q, want %q", filter, "eq=brightness=0.5")
	}

	// Should not resolve non-video plugin as video effect
	plain := &mockPlugin{name: "plain", version: "1.0.0", ext: ExtensionAudioEffect}
	r.Register(plain)
	r.Activate("plain")

	if r.GetVideoEffect("plain") != nil {
		t.Error("GetVideoEffect() should return nil for non-video plugin")
	}

	// Should not resolve inactive plugin
	inactive := &mockVideoEffect{
		mockPlugin: mockPlugin{name: "inactive-effect", version: "1.0.0", ext: ExtensionVideoEffect},
	}
	r.Register(inactive)
	// Don't activate

	if r.GetVideoEffect("inactive-effect") != nil {
		t.Error("GetVideoEffect() should return nil for inactive plugin")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()

	r.Register(&mockPlugin{name: "a", version: "1.0", ext: ExtensionVideoEffect})
	r.Register(&mockPlugin{name: "b", version: "2.0", ext: ExtensionAudioEffect})
	r.Register(&mockPlugin{name: "c", version: "1.0", ext: ExtensionVideoEffect})

	// List all
	all := r.List()
	if len(all) != 3 {
		t.Errorf("List() returned %d plugins, want 3", len(all))
	}

	// List by extension
	videoPlugins := r.ListByExtension(ExtensionVideoEffect)
	if len(videoPlugins) != 2 {
		t.Errorf("ListByExtension(video) returned %d plugins, want 2", len(videoPlugins))
	}

	audioPlugins := r.ListByExtension(ExtensionAudioEffect)
	if len(audioPlugins) != 1 {
		t.Errorf("ListByExtension(audio) returned %d plugins, want 1", len(audioPlugins))
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()

	p := &mockPlugin{name: "removable", version: "1.0.0", ext: ExtensionVideoEffect}
	r.Register(p)
	r.Activate("removable")

	if err := r.Unregister("removable"); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}

	if !p.destroyCalled {
		t.Error("Destroy() should have been called during unregister")
	}

	if r.IsRegistered("removable") {
		t.Error("should not be registered after Unregister()")
	}

	if r.Count() != 0 {
		t.Errorf("Count() = %d, want 0", r.Count())
	}
}

func TestRegistry_ActivateAll(t *testing.T) {
	r := NewRegistry()

	p1 := &mockPlugin{name: "p1", version: "1.0", ext: ExtensionVideoEffect}
	p2 := &mockPlugin{name: "p2", version: "1.0", ext: ExtensionAudioEffect}
	r.Register(p1)
	r.Register(p2)

	if err := r.ActivateAll(); err != nil {
		t.Fatalf("ActivateAll() error = %v", err)
	}

	if !r.IsActive("p1") || !r.IsActive("p2") {
		t.Error("all plugins should be active after ActivateAll()")
	}

	r.DeactivateAll()

	if r.IsActive("p1") || r.IsActive("p2") {
		t.Error("all plugins should be inactive after DeactivateAll()")
	}
}
