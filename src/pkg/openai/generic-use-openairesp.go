package openai

import (
	"encoding/json"
	"os"
	"sort"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"

	"expense-tracker/src/pkg/util"
)

/*
Generic OpenAI responses function. Can be used for everything else.
*/
func UseChatGPTResponsesAPI[T any](
	model string, reasoningEffort Effort,
	instructions, developerMessage, userMessage string, schemaProperties map[string]any,
	maxOutputTokens int, tools []any, toolChoice any,
) (openAIResponse T, llmRunMetadata *LLMRunMetadata, e *xerr.Error) {

	// JSON Schema for Responses API structured outputs
	schema := StrictObj(schemaProperties)
	textOptions := TextAsJSONSchema("schema-name", schema, true)

	inputParameters := InputParameters{
		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
		Model:        model,
		Reasoning:    &Reasoning{Effort: util.Ptr(reasoningEffort)},
		Instructions: instructions,
		Input: []InputItem{
			{Role: RoleDeveloper, Content: developerMessage},
			{Role: RoleUser, Content: userMessage},
		},
		Temperature:     util.Ptr(1.0), // with GPT-5 family have to pass 1.0 or omit. They do not support temperature.
		MaxOutputTokens: &maxOutputTokens,
		Text:            &textOptions,
	}

	if len(tools) > 0 {
		inputParameters.Tools = tools
	}
	if toolChoice != nil {
		inputParameters.ToolChoice = toolChoice
	} else {
		// optional: be explicit; Responses defaults to "auto" if omitted
		inputParameters.ToolChoice = "auto"
	}

	responseText, runMetadata, e := SendPromptReturnResponse(inputParameters)
	if e != nil {
		return openAIResponse, nil, e
	}
	// Report success and echo output
	tl.Log(tl.Info1, palette.Green, "%s id is '%s'", "Received response", runMetadata.ResponseID)
	tl.Log(tl.Verbose, palette.Cyan, "Response text:\n```\n%s\n```", responseText)

	err := json.Unmarshal([]byte(responseText), &openAIResponse)
	if err != nil {
		return openAIResponse, &runMetadata, xerr.NewError(err, "Unable to json.Unmarshal([]byte(responseText), &contactInformation)", responseText)
	}

	return openAIResponse, &runMetadata, nil
}

func GetRequiredFields(schemaProperties map[string]any) []string {
	keys := make([]string, 0, len(schemaProperties))
	for key := range schemaProperties {
		keys = append(keys, key)
	}
	return keys
}

// StrictObj builds a strict JSON Schema "object" where:
// - "properties" = props
// - "additionalProperties" = false
// - "required" = all keys from props (sorted for determinism)
func StrictObj(props map[string]any) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
		"required":             keys,
	}
}
