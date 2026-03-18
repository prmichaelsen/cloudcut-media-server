package edl

import "fmt"

// ValidationError represents a single validation issue.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation issues.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no errors"
	}
	return fmt.Sprintf("%d validation error(s): %s", len(e), e[0].Error())
}

// MediaExistsFn checks whether a media ID exists in storage.
type MediaExistsFn func(mediaID string) bool

// FilterChecker validates filter types and params against registered plugins.
type FilterChecker interface {
	// IsKnownFilter returns true if the filter type is a built-in or registered plugin.
	IsKnownFilter(filterType string) bool
	// ValidateFilterParams validates parameters for a filter type. Returns nil if valid.
	ValidateFilterParams(filterType string, params map[string]interface{}) error
}

// BuiltinFilters is the set of filter types handled natively by the FFmpeg builder.
var BuiltinFilters = map[string]bool{
	"brightness": true,
	"contrast":   true,
	"saturation": true,
	"crop":       true,
	"text":       true,
}

// ValidateWithPlugins checks an EDL for correctness including plugin filter validation.
func ValidateWithPlugins(edl *EDL, mediaExists MediaExistsFn, filterChecker FilterChecker) ValidationErrors {
	errs := Validate(edl, mediaExists)

	// Validate filter references against known filters and plugins
	if filterChecker != nil {
		for i, track := range edl.Timeline.Tracks {
			for j, clip := range track.Clips {
				for k, filter := range clip.Filters {
					filterField := fmt.Sprintf("timeline.tracks[%d].clips[%d].filters[%d]", i, j, k)

					if !BuiltinFilters[filter.Type] && !filterChecker.IsKnownFilter(filter.Type) {
						errs = append(errs, ValidationError{
							Field:   filterField + ".type",
							Message: fmt.Sprintf("unknown filter type %q (not a built-in or registered plugin)", filter.Type),
						})
						continue
					}

					// Validate params against plugin schema
					if !BuiltinFilters[filter.Type] {
						if err := filterChecker.ValidateFilterParams(filter.Type, filter.Params); err != nil {
							errs = append(errs, ValidationError{
								Field:   filterField + ".params",
								Message: err.Error(),
							})
						}
					}
				}
			}
		}
	}

	return errs
}

// Validate checks an EDL for correctness and returns any validation errors.
func Validate(edl *EDL, mediaExists MediaExistsFn) ValidationErrors {
	var errs ValidationErrors

	// Version
	if !isSupportedVersion(edl.Version) {
		errs = append(errs, ValidationError{
			Field:   "version",
			Message: fmt.Sprintf("unsupported version %q (supported: %v)", edl.Version, SupportedVersions),
		})
	}

	// ProjectID
	if edl.ProjectID == "" {
		errs = append(errs, ValidationError{
			Field:   "projectId",
			Message: "projectId is required",
		})
	}

	// Timeline
	if len(edl.Timeline.Tracks) == 0 {
		errs = append(errs, ValidationError{
			Field:   "timeline.tracks",
			Message: "at least one track is required",
		})
	}

	if edl.Timeline.Duration <= 0 {
		errs = append(errs, ValidationError{
			Field:   "timeline.duration",
			Message: "duration must be positive",
		})
	}

	// Tracks and clips
	for i, track := range edl.Timeline.Tracks {
		trackField := fmt.Sprintf("timeline.tracks[%d]", i)

		if track.ID == "" {
			errs = append(errs, ValidationError{
				Field:   trackField + ".id",
				Message: "track id is required",
			})
		}

		if !ValidTrackTypes[track.Type] {
			errs = append(errs, ValidationError{
				Field:   trackField + ".type",
				Message: fmt.Sprintf("invalid track type %q", track.Type),
			})
		}

		for j, clip := range track.Clips {
			clipField := fmt.Sprintf("%s.clips[%d]", trackField, j)
			errs = append(errs, validateClip(clip, clipField, mediaExists)...)
		}
	}

	// Output
	errs = append(errs, validateOutput(edl.Output)...)

	return errs
}

func validateClip(clip Clip, field string, mediaExists MediaExistsFn) ValidationErrors {
	var errs ValidationErrors

	if clip.ID == "" {
		errs = append(errs, ValidationError{
			Field:   field + ".id",
			Message: "clip id is required",
		})
	}

	if clip.MediaID == "" {
		errs = append(errs, ValidationError{
			Field:   field + ".mediaId",
			Message: "mediaId is required",
		})
	} else if mediaExists != nil && !mediaExists(clip.MediaID) {
		errs = append(errs, ValidationError{
			Field:   field + ".mediaId",
			Message: fmt.Sprintf("media %q not found", clip.MediaID),
		})
	}

	if clip.StartTime < 0 {
		errs = append(errs, ValidationError{
			Field:   field + ".startTime",
			Message: "startTime cannot be negative",
		})
	}

	if clip.Duration <= 0 {
		errs = append(errs, ValidationError{
			Field:   field + ".duration",
			Message: "duration must be positive",
		})
	}

	if clip.InPoint < 0 {
		errs = append(errs, ValidationError{
			Field:   field + ".inPoint",
			Message: "inPoint cannot be negative",
		})
	}

	if clip.OutPoint <= clip.InPoint {
		errs = append(errs, ValidationError{
			Field:   field + ".outPoint",
			Message: "outPoint must be greater than inPoint",
		})
	}

	return errs
}

func validateOutput(output Output) ValidationErrors {
	var errs ValidationErrors

	if !ValidOutputFormats[output.Format] {
		errs = append(errs, ValidationError{
			Field:   "output.format",
			Message: fmt.Sprintf("unsupported format %q", output.Format),
		})
	}

	if !ValidQualities[output.Quality] {
		errs = append(errs, ValidationError{
			Field:   "output.quality",
			Message: fmt.Sprintf("unsupported quality %q", output.Quality),
		})
	}

	return errs
}

func isSupportedVersion(version string) bool {
	for _, v := range SupportedVersions {
		if v == version {
			return true
		}
	}
	return false
}
