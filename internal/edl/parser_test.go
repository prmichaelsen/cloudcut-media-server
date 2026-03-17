package edl

import (
	"encoding/json"
	"testing"
)

func TestParse_ValidEDL(t *testing.T) {
	validJSON := `{
		"version": "1.0",
		"projectId": "proj-123",
		"timeline": {
			"duration": 10.5,
			"tracks": [
				{
					"id": "track-1",
					"type": "video",
					"clips": [
						{
							"id": "clip-1",
							"mediaId": "media-1",
							"startTime": 0,
							"duration": 5,
							"inPoint": 0,
							"outPoint": 5
						}
					]
				}
			]
		},
		"output": {
			"format": "mp4",
			"resolution": "1920x1080",
			"codec": "h264",
			"quality": "high"
		}
	}`

	mediaExists := func(mediaID string) bool {
		return mediaID == "media-1"
	}

	edl, errs := Parse([]byte(validJSON), mediaExists)
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
	if edl == nil {
		t.Fatal("expected EDL, got nil")
	}
	if edl.ProjectID != "proj-123" {
		t.Errorf("expected ProjectID 'proj-123', got '%s'", edl.ProjectID)
	}
	if len(edl.Timeline.Tracks) != 1 {
		t.Errorf("expected 1 track, got %d", len(edl.Timeline.Tracks))
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	invalidJSON := `{"version": "1.0", "projectId": `

	edl, errs := Parse([]byte(invalidJSON), nil)
	if edl != nil {
		t.Error("expected nil EDL for invalid JSON")
	}
	if len(errs) == 0 {
		t.Fatal("expected errors for invalid JSON")
	}
	if errs[0].Field != "json" {
		t.Errorf("expected field 'json', got '%s'", errs[0].Field)
	}
}

func TestParse_ValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		wantField string
	}{
		{
			name: "missing projectId",
			json: `{
				"version": "1.0",
				"projectId": "",
				"timeline": {"duration": 10, "tracks": [{"id": "t1", "type": "video", "clips": []}]},
				"output": {"format": "mp4", "quality": "high"}
			}`,
			wantField: "projectId",
		},
		{
			name: "unsupported version",
			json: `{
				"version": "2.0",
				"projectId": "proj-1",
				"timeline": {"duration": 10, "tracks": [{"id": "t1", "type": "video", "clips": []}]},
				"output": {"format": "mp4", "quality": "high"}
			}`,
			wantField: "version",
		},
		{
			name: "no tracks",
			json: `{
				"version": "1.0",
				"projectId": "proj-1",
				"timeline": {"duration": 10, "tracks": []},
				"output": {"format": "mp4", "quality": "high"}
			}`,
			wantField: "timeline.tracks",
		},
		{
			name: "invalid output format",
			json: `{
				"version": "1.0",
				"projectId": "proj-1",
				"timeline": {"duration": 10, "tracks": [{"id": "t1", "type": "video", "clips": []}]},
				"output": {"format": "avi", "quality": "high"}
			}`,
			wantField: "output.format",
		},
		{
			name: "invalid quality",
			json: `{
				"version": "1.0",
				"projectId": "proj-1",
				"timeline": {"duration": 10, "tracks": [{"id": "t1", "type": "video", "clips": []}]},
				"output": {"format": "mp4", "quality": "ultra"}
			}`,
			wantField: "output.quality",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edl, errs := Parse([]byte(tt.json), nil)
			if edl != nil {
				t.Error("expected nil EDL for validation error")
			}
			if len(errs) == 0 {
				t.Fatal("expected validation errors")
			}
			found := false
			for _, err := range errs {
				if err.Field == tt.wantField {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error field '%s', got errors: %v", tt.wantField, errs)
			}
		})
	}
}

func TestParse_MediaNotFound(t *testing.T) {
	validJSON := `{
		"version": "1.0",
		"projectId": "proj-1",
		"timeline": {
			"duration": 10,
			"tracks": [{
				"id": "t1",
				"type": "video",
				"clips": [{
					"id": "c1",
					"mediaId": "media-missing",
					"startTime": 0,
					"duration": 5,
					"inPoint": 0,
					"outPoint": 5
				}]
			}]
		},
		"output": {"format": "mp4", "quality": "high"}
	}`

	mediaExists := func(mediaID string) bool {
		return false
	}

	edl, errs := Parse([]byte(validJSON), mediaExists)
	if edl != nil {
		t.Error("expected nil EDL when media not found")
	}
	if len(errs) == 0 {
		t.Fatal("expected validation errors for missing media")
	}
	found := false
	for _, err := range errs {
		if err.Field == "timeline.tracks[0].clips[0].mediaId" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mediaId error, got: %v", errs)
	}
}

func TestValidate_ClipValidation(t *testing.T) {
	tests := []struct {
		name      string
		clip      Clip
		wantError bool
		wantField string
	}{
		{
			name: "valid clip",
			clip: Clip{
				ID:        "c1",
				MediaID:   "m1",
				StartTime: 0,
				Duration:  5,
				InPoint:   0,
				OutPoint:  5,
			},
			wantError: false,
		},
		{
			name: "negative startTime",
			clip: Clip{
				ID:        "c1",
				MediaID:   "m1",
				StartTime: -1,
				Duration:  5,
				InPoint:   0,
				OutPoint:  5,
			},
			wantError: true,
			wantField: "startTime",
		},
		{
			name: "zero duration",
			clip: Clip{
				ID:        "c1",
				MediaID:   "m1",
				StartTime: 0,
				Duration:  0,
				InPoint:   0,
				OutPoint:  5,
			},
			wantError: true,
			wantField: "duration",
		},
		{
			name: "outPoint <= inPoint",
			clip: Clip{
				ID:        "c1",
				MediaID:   "m1",
				StartTime: 0,
				Duration:  5,
				InPoint:   5,
				OutPoint:  5,
			},
			wantError: true,
			wantField: "outPoint",
		},
		{
			name: "missing mediaId",
			clip: Clip{
				ID:        "c1",
				MediaID:   "",
				StartTime: 0,
				Duration:  5,
				InPoint:   0,
				OutPoint:  5,
			},
			wantError: true,
			wantField: "mediaId",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateClip(tt.clip, "test", func(string) bool { return true })
			if tt.wantError {
				if len(errs) == 0 {
					t.Fatal("expected validation error")
				}
				found := false
				for _, err := range errs {
					if err.Field == "test."+tt.wantField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected field '%s', got errors: %v", tt.wantField, errs)
				}
			} else {
				if len(errs) > 0 {
					t.Errorf("expected no errors, got: %v", errs)
				}
			}
		})
	}
}

func TestValidationErrors_Error(t *testing.T) {
	errs := ValidationErrors{
		{Field: "field1", Message: "error 1"},
		{Field: "field2", Message: "error 2"},
	}

	errStr := errs.Error()
	if errStr != "2 validation error(s): field1: error 1" {
		t.Errorf("unexpected error string: %s", errStr)
	}

	emptyErrs := ValidationErrors{}
	if emptyErrs.Error() != "no errors" {
		t.Errorf("unexpected error string for empty errors: %s", emptyErrs.Error())
	}
}

func TestEDL_JSONMarshaling(t *testing.T) {
	edl := &EDL{
		Version:   "1.0",
		ProjectID: "proj-1",
		Timeline: Timeline{
			Duration: 10.5,
			Tracks: []Track{
				{
					ID:   "t1",
					Type: TrackTypeVideo,
					Clips: []Clip{
						{
							ID:        "c1",
							MediaID:   "m1",
							StartTime: 0,
							Duration:  5,
							InPoint:   0,
							OutPoint:  5,
						},
					},
				},
			},
		},
		Output: Output{
			Format:     "mp4",
			Resolution: "1920x1080",
			Codec:      "h264",
			Quality:    "high",
		},
	}

	data, err := json.Marshal(edl)
	if err != nil {
		t.Fatalf("failed to marshal EDL: %v", err)
	}

	var decoded EDL
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal EDL: %v", err)
	}

	if decoded.ProjectID != edl.ProjectID {
		t.Errorf("projectId mismatch: %s != %s", decoded.ProjectID, edl.ProjectID)
	}
	if len(decoded.Timeline.Tracks) != 1 {
		t.Errorf("expected 1 track, got %d", len(decoded.Timeline.Tracks))
	}
}
