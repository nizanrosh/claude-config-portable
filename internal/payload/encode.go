package payload

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

const (
	Prefix        = "ccfg:"
	SchemaVersion = 1
)

// Encode serializes a ConfigBundle into the wire format: ccfg:1:<base64(gzip(json))>
func Encode(bundle *ConfigBundle) (string, error) {
	data, err := json.Marshal(bundle)
	if err != nil {
		return "", fmt.Errorf("marshaling config bundle: %w", err)
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
	return fmt.Sprintf("%s%d:%s", Prefix, SchemaVersion, encoded), nil
}
