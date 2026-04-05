// SPDX-License-Identifier: MIT

package recommend

// jsonSchema returns the JSON schema string for the RecommendResult type.
// This schema is passed to the claude CLI via --json-schema to enforce
// structured output.
func jsonSchema() string {
	return `{
  "type": "object",
  "properties": {
    "recommendations": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name": { "type": "string" },
          "kind": { "type": "string", "enum": ["skill", "rule", "mcp", "workflow"] },
          "source": { "type": "string" },
          "confidence": { "type": "number", "minimum": 0, "maximum": 1 },
          "reason": { "type": "string" }
        },
        "required": ["name", "kind", "source", "confidence", "reason"]
      }
    }
  },
  "required": ["recommendations"]
}`
}
