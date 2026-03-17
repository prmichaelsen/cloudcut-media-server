package edl

import "time"

// EDL represents an Edit Decision List — the lightweight JSON document
// describing a video editing timeline sent from the client.
type EDL struct {
	Version   string   `json:"version"`
	ProjectID string   `json:"projectId"`
	Timeline  Timeline `json:"timeline"`
	Output    Output   `json:"output"`
}

// Timeline describes the overall editing timeline.
type Timeline struct {
	Duration float64 `json:"duration"` // total duration in seconds
	Tracks   []Track `json:"tracks"`
}

// Track represents a single track (video, audio, or text) on the timeline.
type Track struct {
	ID    string `json:"id"`
	Type  string `json:"type"` // "video", "audio", "text"
	Clips []Clip `json:"clips"`
}

// TrackType constants.
const (
	TrackTypeVideo = "video"
	TrackTypeAudio = "audio"
	TrackTypeText  = "text"
)

// Clip represents a segment of media placed on a track.
type Clip struct {
	ID        string   `json:"id"`
	MediaID   string   `json:"mediaId"`
	StartTime float64  `json:"startTime"` // position on timeline (seconds)
	Duration  float64  `json:"duration"`
	InPoint   float64  `json:"inPoint"`  // source trim start
	OutPoint  float64  `json:"outPoint"` // source trim end
	Filters   []Filter `json:"filters,omitempty"`
}

// Filter represents a video/audio effect applied to a clip.
type Filter struct {
	Type   string                 `json:"type"` // "text", "crop", "brightness", etc.
	Params map[string]interface{} `json:"params"`
}

// Output describes the desired export format.
type Output struct {
	Format     string `json:"format"`     // "mp4"
	Resolution string `json:"resolution"` // "1920x1080", "source"
	Codec      string `json:"codec"`      // "h264"
	Quality    string `json:"quality"`    // "high", "medium", "low"
}

// SupportedVersions lists EDL schema versions the server can process.
var SupportedVersions = []string{"1.0"}

// ValidTrackTypes lists allowed track types.
var ValidTrackTypes = map[string]bool{
	TrackTypeVideo: true,
	TrackTypeAudio: true,
	TrackTypeText:  true,
}

// ValidOutputFormats lists allowed output formats.
var ValidOutputFormats = map[string]bool{
	"mp4":  true,
	"webm": true,
}

// ValidQualities lists allowed quality presets.
var ValidQualities = map[string]bool{
	"high":   true,
	"medium": true,
	"low":    true,
}

// Metadata is attached to a parsed EDL for tracking.
type Metadata struct {
	ReceivedAt time.Time
	SessionID  string
}
