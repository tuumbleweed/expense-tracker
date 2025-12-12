package openai

import (
	"encoding/json"
	"os"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"

	"expense-tracker/src/pkg/util"
)

/*
UseChatGPTResponsesAPIWithImage is similar to UseChatGPTResponsesAPI, but the
user message is sent as structured content with both text and an image.

Parameters:
  - userText: text that accompanies the image (OCR text + price list, etc.).
  - imageDataURL: a data URL string such as "data:image/png;base64,...."
*/
func UseChatGPTResponsesAPIWithImage[T any](
	model string,
	reasoningEffort Effort,
	instructions string,
	developerMessage string,
	userText string,
	imageDataURL string,
	schemaProperties map[string]any,
	maxOutputTokens int,
	tools []any,
	toolChoice any,
) (openAIResponse T, llmRunMetadata *LLMRunMetadata, e *xerr.Error) {

	// JSON Schema for Responses API structured outputs
	schema := StrictObj(schemaProperties)
	textOptions := TextAsJSONSchema("schema-name", schema, true)

	// User content: text + image
	userContent := []map[string]any{
		{
			"type": "input_text",
			"text": userText,
		},
		{
			"type":      "input_image",
			"image_url": imageDataURL,
			// Optionally: "detail": "high",
		},
	}

	inputParameters := InputParameters{
		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
		Model:        model,
		Reasoning:    &Reasoning{Effort: util.Ptr(reasoningEffort)},
		Instructions: instructions,
		Input: []InputItem{
			{
				Role:    RoleDeveloper,
				Content: developerMessage, // plain string is still fine
			},
			{
				Role:    RoleUser,
				Content: userContent, // text + image
			},
		},
		Temperature:     util.Ptr(1.0),
		MaxOutputTokens: &maxOutputTokens,
		Text:            &textOptions,
	}

	if len(tools) > 0 {
		inputParameters.Tools = tools
	}
	if toolChoice != nil {
		inputParameters.ToolChoice = toolChoice
	} else {
		inputParameters.ToolChoice = "auto"
	}

	responseText, runMetadata, e := SendPromptReturnResponse(inputParameters)
	if e != nil {
		return openAIResponse, nil, e
	}

	tl.Log(tl.Info1, palette.Green, "%s id is '%s'", "Received response", runMetadata.ResponseID)
	tl.Log(tl.Verbose, palette.Cyan, "Response text:\n```\n%s\n```", responseText)

	err := json.Unmarshal([]byte(responseText), &openAIResponse)
	if err != nil {
		return openAIResponse, &runMetadata, xerr.NewError(
			err,
			"Unable to json.Unmarshal([]byte(responseText), &openAIResponse)",
			responseText,
		)
	}

	return openAIResponse, &runMetadata, nil
}
