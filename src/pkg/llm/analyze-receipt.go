/*
Parse email reply for contact information (usually in a signature)
*/
package ai

import (
	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"

	"expense-tracker/src/pkg/openai"
)

// Result type for GetReceiptAnalysis responses.
type ReceiptAnalysis struct {
	AIRunMetadata *openai.AIRunMetadata `json:"ai_run_metadata,omitempty"`
}

/*
Take receipt prepared with ocr.
Produce ai.ReceiptAnalysis.
*/
func GenerateReceiptAnalysis(userMessage string, categories map[string]string) (receiptAnalysis ReceiptAnalysis, e *xerr.Error) {

	model := "gpt-5-mini"
	reasoningEffort := openai.EffortLow
	tools := []any{} // disable the tools for now
	toolChoice := "auto"

	tl.Log(
		tl.Notice, palette.BlueBold, "%s with %s model %s, reasoning effort is %s",
		"Generating receipt analysis", "OpenAI", model, reasoningEffort,
	)

	instructions := `
`
	developerMessage := `
`

	// JSON Schema fragment for Responses API (properties only).
	schemaProperties := map[string]any{
	}

	ReceiptAnalysis, aiRunMetadata, e := openai.UseChatGPTResponsesAPI[ReceiptAnalysis](
		model, reasoningEffort, instructions, developerMessage, userMessage, schemaProperties,
		4096, tools, toolChoice,
	)
	ReceiptAnalysis.AIRunMetadata = aiRunMetadata
	if e != nil {
		return ReceiptAnalysis, e
	}

	tl.Log(
		tl.Notice1, palette.GreenBold, "%s with %s model %s, reasoning effort is %s",
		"Generated receipt analysis", "OpenAI", model, reasoningEffort,
	)
	tl.LogJSON(tl.Info, palette.Cyan, "OpenAI Response", schemaProperties)

	return ReceiptAnalysis, nil
}
