// you can add any code you want here but don't commit it.
// keep it empty for future projects and for use ase a template.
package main

import (
	"flag"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"

	"expense-tracker/src/pkg/config"
	"expense-tracker/src/pkg/openai"
)

type OpenAIExampleResponse struct {
	Response string `json:"response"`
}

func main() {
	config.CheckIfEnvVarsPresent("OPENAI_API_KEY")
	// common flags
	configPath := flag.String("config", "./cfg/config.json", "Path to your configuration file.")
	// program's custom flags
	// parse and init config
	flag.Parse()
	config.InitializeConfig(*configPath)

	tl.Log(
		tl.Notice, palette.BlueBold, "%s example entrypoint. Config path: '%s'",
		"Running", *configPath,
	)

	model := "gpt-5-mini"
	reasoningEffort := openai.EffortLow
	maxOutputTokens := 4096
	// tools := []any{openai.NewWebSearchTool()} // if you want to use web search tool - cannot use minimal reasoning effort
	tools := []any{} // disable the tools for now
	toolChoice := "auto"

	instructions := `You need to respond to user prompt`
	developerMessage := `Answer to user message, in this json format: {"response": "<your response>"}`
	userMessage := `Hello, this is a test message`

	schemaProperties := map[string]any{
		"response": map[string]any{"type": "string"},
	}
	openAIExampleResponse, llmRunMetadata, e := openai.UseChatGPTResponsesAPI[OpenAIExampleResponse](
		model, reasoningEffort, instructions, developerMessage, userMessage, schemaProperties,
		maxOutputTokens, tools, toolChoice,
	)
	e.QuitIf("error")
	tl.LogJSON(tl.Notice, palette.Cyan, "Open AI Response", openAIExampleResponse)
	tl.LogJSON(tl.Notice, palette.Cyan, "AI Run Metadata", llmRunMetadata)
}
