package openai

// TextOptions configures output formatting in Responses API.
// Example:
// "text": { "format": { "type": "text" }, "verbosity": "medium" }
// "text": { "format": { "type": "json_object" } }
// "text": { "format": { "type": "json_schema", "json_schema": {...}, "strict": true } }
type TextOptions struct {
	Format    TextFormat    `json:"format"`              // type + optional schema payload
	Verbosity TextVerbosity `json:"verbosity,omitempty"` // optional hint: low|medium|high
}

// TextFormat selects the output format and (for json_schema) carries its config.

// For type == "json_schema", `name` is REQUIRED at this level.
// `json_schema` is just the raw schema object.
// `strict` enforces exact adherence.
type TextFormat struct {
	Type   TextFormatType `json:"type"`             // "text" | "json_object" | "json_schema"
	Name   string         `json:"name,omitempty"`   // REQUIRED when Type == "json_schema"
	Schema map[string]any `json:"schema,omitempty"` // REQUIRED when Type == "json_schema"
	Strict *bool          `json:"strict,omitempty"` // optional; meaningful for json_schema
}

// TextFormatType enumerates supported output formats.
type TextFormatType string

const (
	TextFormatTypeText       TextFormatType = "text"
	TextFormatTypeJSONObject TextFormatType = "json_object"
	TextFormatTypeJSONSchema TextFormatType = "json_schema"
)
