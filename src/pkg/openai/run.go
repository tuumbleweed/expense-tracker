package openai

import (
	"fmt"
	"time"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"

	"expense-tracker/src/pkg/util"
)

/*
SendPromptReturnResponse sends a prompt via the Responses API and returns:
- responseID: the ID of the response object
- responseText: concatenated assistant text from the final response
- e: typed error (nil on success)

Behavior:
1) POST /v1/responses
2) If status != "completed", poll GET /v1/responses/{id} every 2s until a terminal state:
  - "completed"  -> success (green)
  - "failed"|"cancelled"|"expired" -> purple, returns *xerr.Error

3) Log token usage (when available).
NOTE: We purposely DO NOT print the full response text here to avoid duplicate printing.

	The caller (entrypoint) should print responseText.
*/
func SendPromptReturnResponse(inputParameters InputParameters) (responseText string, meta LLMRunMetadata, e *xerr.Error) {
	tl.Log(tl.Info, palette.Blue, "%s %s to %s with previous_response_id='%s'", "Sending", "prompt", "OpenAI Responses API", inputParameters.PreviousResponseID)
	startTime := time.Now()

	requestPayload := requestPayload{
		Model:              inputParameters.Model,
		Reasoning:          inputParameters.Reasoning,
		Store:              true,
		PreviousResponseID: inputParameters.PreviousResponseID,
		Instructions:       inputParameters.Instructions,
		Input:              inputParameters.Input,
		Temperature:        inputParameters.Temperature,
		MaxOutputTokens:    inputParameters.MaxOutputTokens,
		Background:         true, // allows us to poll
		Text:               inputParameters.Text,
		Tools:              inputParameters.Tools,
		ToolChoice:         inputParameters.ToolChoice,
	}

	tl.LogJSON(tl.Debug, palette.CyanDim, "request body", requestPayload)

	initial, createErr := createResponse(inputParameters.OpenAIAPIKey, requestPayload)
	if createErr != nil {
		return "", LLMRunMetadata{}, createErr
	}

	var finalResp responseObject
	switch initial.Status {
	case "", "completed":
		// Completed immediately
		finalResp = initial
	default:
		// Explicit waiting log so the user sees progress right away
		tl.Log(tl.Info, palette.Cyan, "%s current status is '%s' id - '%s' (polling every 2s)...", "Waiting for completion,", initial.Status, initial.ID)
		resp, waitErr := waitForResponseCompletion(inputParameters.OpenAIAPIKey, initial.ID, 2*time.Second, 5*time.Minute)
		if waitErr != nil {
			return "", LLMRunMetadata{ResponseID: initial.ID}, waitErr
		}
		finalResp = resp
	}

	text := extractOutputText(&finalResp)
	meta = ExtractLLMRunMetadata(finalResp, startTime)

	// Token usage logging (if available)
	if finalResp.Usage != nil {
		var cachedTokens, reasoningTokens int
		if finalResp.Usage.InputTokensDetails != nil {
			cachedTokens = finalResp.Usage.InputTokensDetails.CachedTokens
		}
		if finalResp.Usage.InputTokensDetails != nil {
			reasoningTokens = finalResp.Usage.OutputTokensDetails.ReasoningTokens
		}
		tl.Log(
			tl.Detailed,
			palette.CyanDim,
			"Tokens in: %v (cached: %v), out: %v (reasoning: %v), total: %v",
			finalResp.Usage.InputTokens, cachedTokens, finalResp.Usage.OutputTokens,
			reasoningTokens, finalResp.Usage.TotalTokens,
		)
	} else {
		tl.Log(tl.Detailed, palette.PurpleDim, "Usage data is %s", "not available")
	}

	tl.Log(tl.Info1, palette.Green, "%s in %s for the response '%s'", "Response completed", time.Since(startTime), finalResp.ID)
	conversationUrl := fmt.Sprintf("https://platform.openai.com/logs/%s", finalResp.ID)
	tl.Log(tl.Debug1, palette.GreenDim, "You can %s at '%s'", "view conversation URL", conversationUrl)
	// Do NOT log the full text here — leave that to the caller to avoid duplicates.

	util.WaitForSeconds(3)
	return text, meta, nil
}
