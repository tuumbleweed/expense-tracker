package openai

// parameters that our SendPromptReturnResponse function takes
// the reason we are going to use this one instead of requestPayload itself is
// because some of the fields (like Background, MaxOutputTokens, Store, ResponseFormat, Text)
// will be set inside SendPromptReturnResponse and will not be available to control, at least for now.
type InputParameters struct {
	OpenAIAPIKey       string       `json:"open_ai_api_key"` // use OPENAI_API_KEY env var or set another one
	Model              string       `json:"model"`
	Instructions       string       `json:"instructions"`
	MaxOutputTokens    *int         `json:"max_output_tokens,omitempty"`
	Input              []InputItem  `json:"input"`                          // string or []inputItem
	PreviousResponseID string       `json:"previous_response_id,omitempty"` // chain with server-side memory
	Reasoning          *Reasoning   `json:"reasoning"`
	Temperature        *float64     `json:"temperature,omitempty"` // GPT-5 family and o-series do not accept temperature other than 1.0
	Text               *TextOptions `json:"text,omitempty"`
	ToolChoice         any          `json:"tool_choice,omitempty"` // if you need websearch or a custom function
	Tools              []any        `json:"tools,omitempty"`
}

// ----- Request types we send -----

// inputItem is the simplest message shape the Responses API accepts.
// It mirrors examples like: [{"role":"user","content":"..."}]
type InputItem struct {
	Role    InputRole `json:"role"`
	Content any       `json:"content"`
	// Type string `json:"type,omitempty"` // optional; omitted for brevity
}

type requestPayload struct {
	Model              string       `json:"model"`
	Instructions       string       `json:"instructions"`
	MaxOutputTokens    *int         `json:"max_output_tokens,omitempty"`
	Input              []InputItem  `json:"input"`                          // string or []inputItem
	PreviousResponseID string       `json:"previous_response_id,omitempty"` // chain with server-side memory
	Reasoning          *Reasoning   `json:"reasoning"`
	Store              bool         `json:"store,omitempty"` // keep history on OpenAI
	Temperature        *float64     `json:"temperature,omitempty"`
	Background         bool         `json:"background,omitempty"`
	Text               *TextOptions `json:"text,omitempty"`
	ToolChoice         any          `json:"tool_choice,omitempty"` // if you need websearch or a custom function
	Tools              []any        `json:"tools,omitempty"`
}

// ----- Response types we parse -----
/*
Expanded wire structs for the Responses API.
Only includes fields we actually use for ModelRunMetadata.
You can add more later without changing the conversion function.
*/
type responseObject struct {
	ID                 string       `json:"id"`
	Object             string       `json:"object"`
	CreatedAt          int64        `json:"created_at,omitempty"` // epoch seconds (API); we convert to ms
	Background         bool         `json:"background,omitempty"`
	Model              string       `json:"model"`
	Status             string       `json:"status"` // "completed", "in_progress", "failed", etc.
	Output             []outputItem `json:"output"`
	Usage              *usageBlock  `json:"usage,omitempty"`
	PreviousResponseID string       `json:"previous_response_id,omitempty"`
	Error              any          `json:"error,omitempty"`

	// Parameters we care about
	Temperature float64    `json:"temperature,omitempty"`
	Reasoning   *Reasoning `json:"reasoning,omitempty"`
}

type outputItem struct {
	ID      string        `json:"id"`
	Type    string        `json:"type"` // typically "message" or tool events
	Role    string        `json:"role,omitempty"`
	Content []contentItem `json:"content,omitempty"`
}

type contentItem struct {
	Type        string `json:"type"`           // e.g., "output_text"
	Text        string `json:"text,omitempty"` // set when type == "output_text"
	Annotations []any  `json:"annotations,omitempty"`
	Logprobs    []any  `json:"logprobs,omitempty"`
}

type usageBlock struct {
	InputTokens         int                  `json:"input_tokens"`
	InputTokensDetails  *inputTokensDetails  `json:"input_tokens_details"`
	OutputTokens        int                  `json:"output_tokens"`
	TotalTokens         int                  `json:"total_tokens"`
	OutputTokensDetails *outputTokensDetails `json:"output_tokens_details,omitempty"`
}

type inputTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type outputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

type Reasoning struct {
	Effort  *Effort  `json:"effort,omitempty"`
	Summary *Summary `json:"summary,omitempty"` // Your organization must be verified to generate reasoning summaries.
}

// ModelRunMetadata captures how an AI response was generated.
// Keep it alongside your result payload for auditing and cost tracking.
type AIRunMetadata struct {
	// Core
	ResponseID      string `json:"response_id"`       // can make url out of it to see it at https://platform.openai.com/logs/<ResponseID>
	ResponseLogsUrl string `json:"response_logs_url"` // https://platform.openai.com/logs/<ResponseID>
	Model           string `json:"model"`             // e.g., "gpt-5-mini"
	ModelSnapshot   string `json:"model_snapshot"`    // Parsed snapshot date, e.g. "2025-08-07" if present/valid
	Status          string `json:"status"`            // response.status (e.g., "completed")
	ReasoningEffort Effort `json:"reasoning_effort"`  // "minimal" | "low" | "medium" | "high"

	// Parameters
	Temperature float64 `json:"temperature"`

	// Token accounting
	TokensIn        int `json:"tokens_in"`     // instructions, developer messages, schemas, user messages
	TokensCached    int `json:"tokens_cached"` // amount of cached tokens used (it's cheaper than in)
	TokensOut       int `json:"tokens_out"`
	TokensReasoning int `json:"tokens_reasoning"` // tokens spent on reasoning
	TokensTotal     int `json:"tokens_total"`

	// Timing & IDs
	StartedAt  int64 `json:"started_at"`
	FinishedAt int64 `json:"finished_at"`
	Elapsed    int64 `json:"elapsed"` // milliseconds
}
