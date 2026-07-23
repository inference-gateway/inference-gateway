package codegen

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// GenerateMCPWrap reads the raw MCP JSON Schema (draft 2020-12, top-level
// `$defs`) and writes a minimal OpenAPI 3.1 document that oapi-codegen can
// consume: `$defs` becomes `components.schemas` and every `#/$defs/...` ref is
// rewritten to `#/components/schemas/...`. It is a deterministic transform with
// no new dependency (gopkg.in/yaml.v3 is already vendored).
func GenerateMCPWrap(output, input string) error {
	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("reading MCP schema %s: %w", input, err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing MCP schema: %w", err)
	}

	schemas, ok := doc["$defs"]
	if !ok {
		return fmt.Errorf("MCP schema %s has no top-level $defs", input)
	}

	// oapi-codegen v2.8 does not support JSON Schema 2020-12 multi-type arrays
	// (e.g. `type: [string, integer, boolean]`). Drop the `type` on those nodes
	// so they generate as `any`, matching the previous generator's handling of
	// JSONValue/RequestId/ProgressToken.
	dropMultiTypeArrays(schemas)

	// Pin the types the internal/mcp consumers treat loosely to the same shapes
	// the previous generator produced, so migrating tools -> oapi-codegen is a
	// no-op for those files:
	//   - arbitrary-JSON objects (`additionalProperties: {}`) -> map[string]any
	//     (oapi-codegen would otherwise mint a named *_InputSchema type / pointer)
	//   - the ContentBlock union -> any (consumers assign raw maps into it)
	annotateLooseObjects(schemas)
	if cb, ok := schemas.(map[string]any)["ContentBlock"].(map[string]any); ok {
		cb["x-go-type"] = "interface{}"
	}

	wrapped := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "MCP JSON-RPC Schema",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
		"components": map[string]any{
			"schemas": schemas,
		},
	}

	out, err := yaml.Marshal(wrapped)
	if err != nil {
		return fmt.Errorf("marshalling wrapped schema: %w", err)
	}
	out = bytes.ReplaceAll(out, []byte("#/$defs/"), []byte("#/components/schemas/"))

	if err := os.WriteFile(output, out, 0o644); err != nil {
		return fmt.Errorf("writing wrapped schema %s: %w", output, err)
	}
	return nil
}

// annotateLooseObjects walks the schema tree and, for every object schema whose
// `additionalProperties` is the empty schema (`{}`, i.e. an arbitrary JSON
// object), pins it to `map[string]interface{}` with no optional pointer. This
// reproduces the previous generator's output and keeps consumers unchanged.
func annotateLooseObjects(node any) {
	switch v := node.(type) {
	case map[string]any:
		if v["type"] == "object" {
			if ap, ok := v["additionalProperties"].(map[string]any); ok && len(ap) == 0 {
				v["x-go-type"] = "map[string]interface{}"
				v["x-go-type-skip-optional-pointer"] = true
			}
		}
		for _, child := range v {
			annotateLooseObjects(child)
		}
	case []any:
		for _, child := range v {
			annotateLooseObjects(child)
		}
	}
}

// dropMultiTypeArrays walks the schema tree and removes any `type` key whose
// value is a list (JSON Schema multi-type). oapi-codegen cannot render these.
func dropMultiTypeArrays(node any) {
	switch v := node.(type) {
	case map[string]any:
		if _, isList := v["type"].([]any); isList {
			delete(v, "type")
		}
		for _, child := range v {
			dropMultiTypeArrays(child)
		}
	case []any:
		for _, child := range v {
			dropMultiTypeArrays(child)
		}
	}
}
