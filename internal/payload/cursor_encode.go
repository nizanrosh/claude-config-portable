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

const CursorPrefix = "ccur:"

// EncodeCursor serializes a CursorConfigBundle into the wire format: ccur:1:<base64(gzip(json))>
func EncodeCursor(bundle *CursorConfigBundle) (string, error) {
	data, err := json.Marshal(bundle)
	if err != nil {
		return "", fmt.Errorf("marshaling cursor config bundle: %w", err)
	}

	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return "", fmt.Errorf("creating gzip writer: %w", err)
	}
	if _, err := gz.Write(data); err != nil {
		return "", fmt.Errorf("gzip compressing: %w", err)
	}
	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("closing gzip writer: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return fmt.Sprintf("%s%d:%s", CursorPrefix, CursorSchemaVersion, encoded), nil
}

// DecodeCursor parses a wire-format string back into a CursorConfigBundle.
func DecodeCursor(input string) (*CursorConfigBundle, error) {
	if !strings.HasPrefix(input, CursorPrefix) {
		return nil, fmt.Errorf("invalid format: missing %q prefix", CursorPrefix)
	}

	rest := strings.TrimPrefix(input, CursorPrefix)
	colonIdx := strings.Index(rest, ":")
	if colonIdx < 0 {
		return nil, fmt.Errorf("invalid format: missing version separator")
	}

	versionStr := rest[:colonIdx]
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version %q: %w", versionStr, err)
	}
	if version != CursorSchemaVersion {
		return nil, fmt.Errorf("unsupported schema version %d (expected %d)", version, CursorSchemaVersion)
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

	var bundle CursorConfigBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("unmarshaling cursor config bundle: %w", err)
	}

	return &bundle, nil
}
