package edl

import (
	"encoding/json"
	"fmt"
)

// Parse unmarshals JSON bytes into an EDL and validates it.
// Returns the parsed EDL or validation errors.
func Parse(data []byte, mediaExists MediaExistsFn) (*EDL, ValidationErrors) {
	var edl EDL
	if err := json.Unmarshal(data, &edl); err != nil {
		return nil, ValidationErrors{{
			Field:   "json",
			Message: fmt.Sprintf("invalid JSON: %v", err),
		}}
	}

	if errs := Validate(&edl, mediaExists); len(errs) > 0 {
		return nil, errs
	}

	return &edl, nil
}
