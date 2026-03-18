package render

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/prmichaelsen/cloudcut-media-server/internal/edl"
	"github.com/prmichaelsen/cloudcut-media-server/internal/plugin"
	"github.com/prmichaelsen/cloudcut-media-server/internal/progress"
)

// FFmpegRenderer implements FFmpegClient for building and executing FFmpeg commands.
type FFmpegRenderer struct {
	ffmpegPath     string
	pluginRegistry *plugin.Registry
}

// NewFFmpegRenderer creates a new FFmpegRenderer.
func NewFFmpegRenderer(ffmpegPath string) *FFmpegRenderer {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	return &FFmpegRenderer{
		ffmpegPath: ffmpegPath,
	}
}

// SetPluginRegistry sets the plugin registry for resolving plugin filters.
func (f *FFmpegRenderer) SetPluginRegistry(registry *plugin.Registry) {
	f.pluginRegistry = registry
}

// BuildRenderCommand constructs FFmpeg arguments from an EDL.
func (f *FFmpegRenderer) BuildRenderCommand(edl *edl.EDL, inputPaths []string, outputPath string) ([]string, error) {
	args := []string{"-y"} // overwrite output

	// Add input files
	for _, path := range inputPaths {
		args = append(args, "-i", path)
	}

	// Build filter complex for timeline
	filterComplex, err := f.buildFilterComplex(edl)
	if err != nil {
		return nil, fmt.Errorf("build filter complex: %w", err)
	}

	if filterComplex != "" {
		args = append(args, "-filter_complex", filterComplex)
	}

	// Output settings based on EDL
	args = append(args, f.buildOutputArgs(edl)...)
	args = append(args, outputPath)

	return args, nil
}

// Run executes FFmpeg with progress tracking.
func (f *FFmpegRenderer) Run(ctx context.Context, args []string, progressCb ProgressCallback) error {
	// Estimate total duration from input files if possible
	// For now, we'll parse progress without percentage (percentages calculated by parser)
	// In a full implementation, we'd probe input files to get total duration

	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)

	// Capture stderr for progress parsing
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create stderr pipe: %w", err)
	}

	// Start FFmpeg
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	// Parse progress in a goroutine
	done := make(chan error, 1)
	go func() {
		parser := progress.NewParser(0) // We don't know total duration yet
		err := parser.Parse(stderrPipe, func(prog progress.Progress) {
			if progressCb != nil {
				// Convert speed to string format
				speedStr := ""
				if prog.Speed > 0 {
					speedStr = fmt.Sprintf("%.1fx", prog.Speed)
				}

				// Calculate rough ETA (in seconds)
				// Without knowing total duration, we can't calculate accurate ETA
				// This would be improved with ffprobe duration lookup
				eta := 0

				progressCb(prog.Progress, int(prog.FPS), speedStr, eta, "rendering")
			}
		})
		done <- err
	}()

	// Wait for FFmpeg to complete
	cmdErr := cmd.Wait()

	// Wait for progress parser to finish
	<-done

	if cmdErr != nil {
		return fmt.Errorf("ffmpeg failed: %w", cmdErr)
	}

	return nil
}

// buildFilterComplex constructs the filter_complex graph for the EDL timeline.
func (f *FFmpegRenderer) buildFilterComplex(parsedEDL *edl.EDL) (string, error) {
	var filters []string

	// Group clips by track type
	var videoTracks []edl.Track
	var audioTracks []edl.Track
	for _, track := range parsedEDL.Timeline.Tracks {
		switch track.Type {
		case "video":
			videoTracks = append(videoTracks, track)
		case "audio":
			audioTracks = append(audioTracks, track)
		}
	}

	// Build video filter chain
	if len(videoTracks) > 0 {
		videoFilter, err := f.buildVideoFilter(videoTracks, parsedEDL)
		if err != nil {
			return "", fmt.Errorf("build video filter: %w", err)
		}
		filters = append(filters, videoFilter)
	}

	// Build audio filter chain
	if len(audioTracks) > 0 {
		audioFilter := f.buildAudioFilter(audioTracks)
		filters = append(filters, audioFilter)
	}

	return strings.Join(filters, ";"), nil
}

// buildVideoFilter creates the video filter chain with trim, scale, concat, and effects.
func (f *FFmpegRenderer) buildVideoFilter(tracks []edl.Track, parsedEDL *edl.EDL) (string, error) {
	// For MVP, handle single video track with multiple clips
	if len(tracks) != 1 {
		return "", fmt.Errorf("multi-track video not yet supported (got %d tracks)", len(tracks))
	}

	track := tracks[0]
	if len(track.Clips) == 0 {
		return "", fmt.Errorf("video track has no clips")
	}

	var segments []string
	resolution := parsedEDL.Output.Resolution

	// Process each clip: trim, scale, apply filters
	for i, clip := range track.Clips {
		label := fmt.Sprintf("v%d", i)

		// Trim clip to in/out points
		filter := fmt.Sprintf("[%d:v]trim=start=%.3f:end=%.3f,setpts=PTS-STARTPTS",
			i, clip.InPoint, clip.OutPoint)

		// Scale to output resolution
		if resolution != "" && resolution != "source" {
			filter += fmt.Sprintf(",scale=%s", resolution)
		}

		// Apply clip filters
		for _, clipFilter := range clip.Filters {
			filterStr := f.buildFilterString(clipFilter)
			if filterStr != "" {
				filter += "," + filterStr
			}
		}

		filter += fmt.Sprintf("[%s]", label)
		segments = append(segments, filter)
	}

	// Concatenate all segments
	concatInput := ""
	for i := range track.Clips {
		concatInput += fmt.Sprintf("[v%d]", i)
	}
	concat := fmt.Sprintf("%sconcat=n=%d:v=1:a=0[vout]", concatInput, len(track.Clips))
	segments = append(segments, concat)

	return strings.Join(segments, ";"), nil
}

// buildAudioFilter creates the audio filter chain with trim and concat.
func (f *FFmpegRenderer) buildAudioFilter(tracks []edl.Track) string {
	// For MVP, handle single audio track
	if len(tracks) != 1 {
		return ""
	}

	track := tracks[0]
	if len(track.Clips) == 0 {
		return ""
	}

	var segments []string

	// Trim each audio clip
	for i, clip := range track.Clips {
		label := fmt.Sprintf("a%d", i)
		filter := fmt.Sprintf("[%d:a]atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS[%s]",
			i, clip.InPoint, clip.OutPoint, label)
		segments = append(segments, filter)
	}

	// Concatenate all segments
	concatInput := ""
	for i := range track.Clips {
		concatInput += fmt.Sprintf("[a%d]", i)
	}
	concat := fmt.Sprintf("%sconcat=n=%d:v=0:a=1[aout]", concatInput, len(track.Clips))
	segments = append(segments, concat)

	return strings.Join(segments, ";")
}

// buildOutputArgs constructs output encoding arguments from EDL.
func (f *FFmpegRenderer) buildOutputArgs(parsedEDL *edl.EDL) []string {
	args := []string{"-map", "[vout]"}

	// Add audio if present
	hasAudio := false
	for _, track := range parsedEDL.Timeline.Tracks {
		if track.Type == "audio" && len(track.Clips) > 0 {
			hasAudio = true
			break
		}
	}
	if hasAudio {
		args = append(args, "-map", "[aout]")
	}

	// Codec settings
	codec := parsedEDL.Output.Codec
	if codec == "" {
		codec = "h264"
	}

	switch codec {
	case "h264":
		args = append(args, "-c:v", "libx264")
		args = append(args, f.buildH264Args(parsedEDL.Output.Quality)...)
	case "h265":
		args = append(args, "-c:v", "libx265")
		args = append(args, f.buildH265Args(parsedEDL.Output.Quality)...)
	}

	if hasAudio {
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	}

	// Format-specific settings
	switch parsedEDL.Output.Format {
	case "mp4":
		args = append(args, "-movflags", "+faststart")
	case "webm":
		args = append(args, "-c:v", "libvpx-vp9")
	}

	return args
}

// buildH264Args returns H.264 encoding args based on quality preset.
func (f *FFmpegRenderer) buildH264Args(quality string) []string {
	switch quality {
	case "high":
		return []string{"-preset", "slow", "-crf", "18"}
	case "medium":
		return []string{"-preset", "medium", "-crf", "23"}
	case "low":
		return []string{"-preset", "fast", "-crf", "28"}
	default:
		return []string{"-preset", "medium", "-crf", "23"}
	}
}

// buildH265Args returns H.265 encoding args based on quality preset.
func (f *FFmpegRenderer) buildH265Args(quality string) []string {
	switch quality {
	case "high":
		return []string{"-preset", "slow", "-crf", "22"}
	case "medium":
		return []string{"-preset", "medium", "-crf", "26"}
	case "low":
		return []string{"-preset", "fast", "-crf", "30"}
	default:
		return []string{"-preset", "medium", "-crf", "26"}
	}
}

// buildFilterString converts a Filter to FFmpeg filter syntax.
// It checks built-in filters first, then falls back to the plugin registry.
func (r *FFmpegRenderer) buildFilterString(f edl.Filter) string {
	// Try built-in filters first
	switch f.Type {
	case "brightness":
		if val, ok := f.Params["value"].(float64); ok {
			return fmt.Sprintf("eq=brightness=%.2f", val)
		}
	case "contrast":
		if val, ok := f.Params["value"].(float64); ok {
			return fmt.Sprintf("eq=contrast=%.2f", val)
		}
	case "saturation":
		if val, ok := f.Params["value"].(float64); ok {
			return fmt.Sprintf("eq=saturation=%.2f", val)
		}
	case "crop":
		x, _ := f.Params["x"].(float64)
		y, _ := f.Params["y"].(float64)
		w, _ := f.Params["width"].(float64)
		h, _ := f.Params["height"].(float64)
		return fmt.Sprintf("crop=%.0f:%.0f:%.0f:%.0f", w, h, x, y)
	case "text":
		text, _ := f.Params["text"].(string)
		x, _ := f.Params["x"].(float64)
		y, _ := f.Params["y"].(float64)
		size, _ := f.Params["size"].(float64)
		if size == 0 {
			size = 24
		}
		return fmt.Sprintf("drawtext=text='%s':x=%.0f:y=%.0f:fontsize=%.0f:fontcolor=white",
			strings.ReplaceAll(text, "'", "\\'"), x, y, size)
	}

	// Try plugin registry
	if r.pluginRegistry != nil {
		vp := r.pluginRegistry.GetVideoEffect(f.Type)
		if vp != nil {
			filterStr, err := vp.BuildFFmpegFilter(f.Params)
			if err != nil {
				log.Printf("plugin %s BuildFFmpegFilter error: %v", f.Type, err)
				return ""
			}
			return filterStr
		}
	}

	return ""
}
