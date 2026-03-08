package merge

import (
	"encoding/json"
	"testing"
)

func TestDeepMerge_Objects(t *testing.T) {
	t.Parallel()

	base := json.RawMessage(`{"a":"1","b":{"x":"old","y":"keep"}}`)
	overlay := json.RawMessage(`{"a":"2","b":{"x":"new","z":"added"},"c":"3"}`)

	result, err := DeepMerge(base, overlay)
	if err != nil {
		t.Fatalf("DeepMerge failed: %v", err)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}

	// "a" should be overwritten by overlay
	var a string
	if err := json.Unmarshal(obj["a"], &a); err != nil {
		t.Fatalf("unmarshal a failed: %v", err)
	}
	if a != "2" {
		t.Errorf("a should be overwritten: got %q", a)
	}

	// "c" should be added from overlay
	var c string
	if err := json.Unmarshal(obj["c"], &c); err != nil {
		t.Fatalf("unmarshal c failed: %v", err)
	}
	if c != "3" {
		t.Errorf("c should be added: got %q", c)
	}

	// nested "b" should be merged
	var b map[string]string
	if err := json.Unmarshal(obj["b"], &b); err != nil {
		t.Fatalf("unmarshal b failed: %v", err)
	}
	if b["x"] != "new" {
		t.Errorf("b.x should be overwritten: got %q", b["x"])
	}
	if b["y"] != "keep" {
		t.Errorf("b.y should be kept: got %q", b["y"])
	}
	if b["z"] != "added" {
		t.Errorf("b.z should be added: got %q", b["z"])
	}
}

func TestDeepMerge_NilInputs(t *testing.T) {
	t.Parallel()

	data := json.RawMessage(`{"key":"value"}`)

	// nil base
	result, err := DeepMerge(nil, data)
	if err != nil {
		t.Fatalf("DeepMerge(nil, data) failed: %v", err)
	}
	if string(result) != string(data) {
		t.Errorf("nil base should return overlay: got %s", result)
	}

	// nil overlay
	result, err = DeepMerge(data, nil)
	if err != nil {
		t.Fatalf("DeepMerge(data, nil) failed: %v", err)
	}
	if string(result) != string(data) {
		t.Errorf("nil overlay should return base: got %s", result)
	}
}

func TestDeepMerge_NonObjectOverlay(t *testing.T) {
	t.Parallel()

	base := json.RawMessage(`{"a":"1"}`)
	overlay := json.RawMessage(`["array"]`)

	result, err := DeepMerge(base, overlay)
	if err != nil {
		t.Fatalf("DeepMerge failed: %v", err)
	}

	// overlay wins when types differ
	if string(result) != `["array"]` {
		t.Errorf("non-object overlay should win: got %s", result)
	}
}
