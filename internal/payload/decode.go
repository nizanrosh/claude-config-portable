package payload

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Decode parses a wire-format string back into a ConfigBundle.
func Decode(input string) (*ConfigBundle, error) {
	if !strings.HasPrefix(input, Prefix) {
		return nil, fmt.Errorf("invalid format: missing %q prefix", Prefix)
	}

	rest := strings.TrimPrefix(input, Prefix)
	colonIdx := strings.Index(rest, ":")
	if colonIdx < 0 {
		return nil, fmt.Errorf("invalid format: missing version separator")
	}

	versionStr := rest[:colonIdx]
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version %q: %w", versionStr, err)
	}
	if version != SchemaVersion {
		return nil, fmt.Errorf("unsupported schema version %d (expected %d)", version, SchemaVersion)
	}

	b64Data := rest[colonIdx+1:]
	compressed, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return nil, fmt.Errorf("base64 decoding: %w", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gz.Close()

	data, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("decompressing: %w", err)
	}

	var bundle ConfigBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("unmarshaling config bundle: %w", err)
	}

	return &bundle, nil
}
