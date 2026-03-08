// Package merge provides recursive JSON deep-merge for the --merge import mode.
package merge

import (
	"encoding/json"
	"fmt"
)

// DeepMerge merges two JSON documents. Values from overlay take precedence
// on conflicts. Objects are recursively merged; all other types are overwritten.
func DeepMerge(base, overlay json.RawMessage) (json.RawMessage, error) {
	if base == nil || len(base) == 0 {
		return overlay, nil
	}
	if overlay == nil || len(overlay) == 0 {
		return base, nil
	}

	var baseObj map[string]json.RawMessage
	var overlayObj map[string]json.RawMessage

	baseIsObj := json.Unmarshal(base, &baseObj) == nil
	overlayIsObj := json.Unmarshal(overlay, &overlayObj) == nil

	// Both are objects — recurse
	if baseIsObj && overlayIsObj {
		for key, val := range overlayObj {
			if existing, ok := baseObj[key]; ok {
				merged, err := DeepMerge(existing, val)
				if err != nil {
					return nil, fmt.Errorf("merging key %q: %w", key, err)
				}
				baseObj[key] = merged
			} else {
				baseObj[key] = val
			}
		}
		out, err := json.Marshal(baseObj)
		if err != nil {
			return nil, fmt.Errorf("marshaling merged object: %w", err)
		}
		return out, nil
	}

	// Not both objects — overlay wins
	return overlay, nil
}
