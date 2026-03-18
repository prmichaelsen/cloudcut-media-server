package filmgrain

import (
	"testing"

	"github.com/prmichaelsen/cloudcut-media-server/internal/plugin"
)

func TestFilmGrainPlugin_Interface(t *testing.T) {
	p := &FilmGrainPlugin{}

	if p.Name() != "film-grain" {
		t.Errorf("Name() = %q, want %q", p.Name(), "film-grain")
	}
	if p.Version() != "1.0.0" {
		t.Errorf("Version() = %q, want %q", p.Version(), "1.0.0")
	}
	if p.ExtensionPoint() != plugin.ExtensionVideoEffect {
		t.Errorf("ExtensionPoint() = %q, want %q", p.ExtensionPoint(), plugin.ExtensionVideoEffect)
	}

	if err := p.Init(); err != nil {
		t.Errorf("Init() error = %v", err)
	}
	if err := p.Destroy(); err != nil {
		t.Errorf("Destroy() error = %v", err)
	}
}

func TestFilmGrainPlugin_BuildFFmpegFilter(t *testing.T) {
	p := &FilmGrainPlugin{}

	tests := []struct {
		name   string
		params map[string]interface{}
		want   string
	}{
		{
			name:   "default intensity",
			params: map[string]interface{}{},
			want:   "noise=alls=15:allf=t+u",
		},
		{
			name:   "zero intensity",
			params: map[string]interface{}{"intensity": 0.0},
			want:   "noise=alls=0:allf=t+u",
		},
		{
			name:   "full intensity",
			params: map[string]interface{}{"intensity": 1.0},
			want:   "noise=alls=30:allf=t+u",
		},
		{
			name:   "half intensity",
			params: map[string]interface{}{"intensity": 0.5},
			want:   "noise=alls=15:allf=t+u",
		},
		{
			name:   "low intensity",
			params: map[string]interface{}{"intensity": 0.1},
			want:   "noise=alls=3:allf=t+u",
		},
		{
			name:   "clamp over 1",
			params: map[string]interface{}{"intensity": 2.0},
			want:   "noise=alls=30:allf=t+u",
		},
		{
			name:   "clamp under 0",
			params: map[string]interface{}{"intensity": -1.0},
			want:   "noise=alls=0:allf=t+u",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.BuildFFmpegFilter(tt.params)
			if err != nil {
				t.Fatalf("BuildFFmpegFilter() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("BuildFFmpegFilter() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilmGrainPlugin_ValidateParams(t *testing.T) {
	p := &FilmGrainPlugin{}

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid params",
			params:  map[string]interface{}{"intensity": 0.5, "size": 1.0},
			wantErr: false,
		},
		{
			name:    "empty params",
			params:  map[string]interface{}{},
			wantErr: false,
		},
		{
			name:    "intensity too high",
			params:  map[string]interface{}{"intensity": 1.5},
			wantErr: true,
		},
		{
			name:    "intensity too low",
			params:  map[string]interface{}{"intensity": -0.5},
			wantErr: true,
		},
		{
			name:    "intensity wrong type",
			params:  map[string]interface{}{"intensity": "high"},
			wantErr: true,
		},
		{
			name:    "size too small",
			params:  map[string]interface{}{"size": 0.01},
			wantErr: true,
		},
		{
			name:    "size too large",
			params:  map[string]interface{}{"size": 10.0},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.ValidateParams(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateParams() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestFilmGrainPlugin_DefaultParams(t *testing.T) {
	p := &FilmGrainPlugin{}
	defaults := p.DefaultParams()

	if defaults["intensity"] != 0.5 {
		t.Errorf("default intensity = %v, want 0.5", defaults["intensity"])
	}
	if defaults["size"] != 1.0 {
		t.Errorf("default size = %v, want 1.0", defaults["size"])
	}
}

func TestFilmGrainPlugin_RegisterWithRegistry(t *testing.T) {
	r := plugin.NewRegistry()
	p := &FilmGrainPlugin{}

	if err := r.Register(p); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if err := r.Activate("film-grain"); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	vp := r.GetVideoEffect("film-grain")
	if vp == nil {
		t.Fatal("GetVideoEffect() returned nil")
	}

	filter, err := vp.BuildFFmpegFilter(map[string]interface{}{"intensity": 0.7})
	if err != nil {
		t.Fatalf("BuildFFmpegFilter() error = %v", err)
	}
	if filter != "noise=alls=21:allf=t+u" {
		t.Errorf("filter = %q, want %q", filter, "noise=alls=21:allf=t+u")
	}
}
