package secrets

import (
	"encoding/json"
	"fmt"
	"net/url"
)

const redactedPlaceholder = "__CONFIGURE_AFTER_IMPORT__"

// sensitiveKeys are the top-level MCP config keys that may contain secrets.
var sensitiveKeys = []string{"headers", "env", "oauth"}

// FilterMCPConfig removes secret-bearing blocks from an MCP config JSON object.
// It replaces headers, env, and oauth with a placeholder string and strips
// query parameters from url values.
func FilterMCPConfig(raw json.RawMessage) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return raw, nil // not an object, return as-is
	}

	changed := false
	for _, key := range sensitiveKeys {
		if _, ok := obj[key]; ok {
			placeholder, _ := json.Marshal(redactedPlaceholder)
			obj[key] = placeholder
			changed = true
		}
	}

	if urlRaw, ok := obj["url"]; ok {
		filtered, didFilter, err := filterURL(urlRaw)
		if err == nil && didFilter {
			obj["url"] = filtered
			changed = true
		}
	}

	// Recurse into nested "mcpServers" maps (common in .mcp.json files)
	if servers, ok := obj["mcpServers"]; ok {
		filtered, err := filterServersMap(servers)
		if err == nil {
			obj["mcpServers"] = filtered
			changed = true
		}
	}

	if !changed {
		return raw, nil
	}

	out, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("re-marshaling filtered MCP config: %w", err)
	}
	return out, nil
}

// FilterMCPServers filters all servers in a top-level MCP config that has
// a "mcpServers" key (the format used by ~/.claude/.mcp.json).
func FilterMCPServers(raw json.RawMessage) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return raw, nil
	}

	if servers, ok := obj["mcpServers"]; ok {
		filtered, err := filterServersMap(servers)
		if err != nil {
			return nil, err
		}
		obj["mcpServers"] = filtered
	}

	out, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("re-marshaling filtered MCP servers: %w", err)
	}
	return out, nil
}

func filterServersMap(raw json.RawMessage) (json.RawMessage, error) {
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(raw, &servers); err != nil {
		return raw, nil
	}

	for name, serverCfg := range servers {
		filtered, err := FilterMCPConfig(serverCfg)
		if err != nil {
			return nil, fmt.Errorf("filtering MCP server %q: %w", name, err)
		}
		servers[name] = filtered
	}

	out, err := json.Marshal(servers)
	if err != nil {
		return nil, fmt.Errorf("re-marshaling servers map: %w", err)
	}
	return out, nil
}

// filterURL strips query parameters from a JSON-encoded URL string.
func filterURL(raw json.RawMessage) (json.RawMessage, bool, error) {
	var u string
	if err := json.Unmarshal(raw, &u); err != nil {
		return raw, false, nil // not a string
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return raw, false, nil
	}
	if parsed.RawQuery == "" {
		return raw, false, nil
	}
	parsed.RawQuery = ""
	out, err := json.Marshal(parsed.String())
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

// HasRedactedValues checks whether a MCP config contains the redaction placeholder.
func HasRedactedValues(raw json.RawMessage) bool {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	for _, key := range sensitiveKeys {
		if val, ok := obj[key]; ok {
			var s string
			if json.Unmarshal(val, &s) == nil && s == redactedPlaceholder {
				return true
			}
		}
	}
	// Check nested mcpServers
	if servers, ok := obj["mcpServers"]; ok {
		var srvMap map[string]json.RawMessage
		if json.Unmarshal(servers, &srvMap) == nil {
			for _, srv := range srvMap {
				if HasRedactedValues(srv) {
					return true
				}
			}
		}
	}
	return false
}
