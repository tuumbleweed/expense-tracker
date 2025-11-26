// you can add any code you want here but don't commit it.
// keep it empty for future projects and for use ase a template.
package main

import (
	"flag"
	"os"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"

	"expense-tracker/src/pkg/config"
	"expense-tracker/src/pkg/llm"
	"expense-tracker/src/pkg/util"
)

/*
main is the entrypoint for this example program.

It initializes configuration, parses flags, reads an OCR text file produced
by the OCR pipeline, and uses llm.GenerateReceiptAnalysis to turn that text
into a structured receipt analysis. Errors are printed and cause a non-zero
exit code.
*/
func main() {
	// Ensure required environment variables are present.
	config.CheckIfEnvVarsPresent("OPENAI_API_KEY")
	// Common flags.
	configPath := flag.String("config", "./cfg/config.json", "Path to your configuration file.")
	// Program-specific flags.
	ocrTextPath := flag.String("ocr-text", "", "Path to the OCR text file to analyze.")
	// Parse flags and initialize config.
	flag.Parse()
	// Mark required flags.
	util.RequiredFlag(ocrTextPath, "ocr-text")
	util.EnsureFlags()
	config.InitializeConfig(*configPath)

	tl.Log(
		tl.Notice, palette.BlueBold, "%s entrypoint. Config path: '%s'",
		"Running receipt analysis", *configPath,
	)

	// Read OCR text from the provided file path.
	ocrBytes, readErr := os.ReadFile(*ocrTextPath)
	xerr.QuitIfError(readErr, "read OCR text file")
	ocrText := string(ocrBytes)

	tl.Log(
		tl.Info1, palette.Cyan, "Loaded OCR text from '%s' (length: %s)",
		*ocrTextPath, len(ocrText),
	)

	// For now, pass nil to use the default category map inside the LLM layer.
	receiptAnalysis, analysisErr := llm.GenerateReceiptAnalysis(ocrText, nil)
	if analysisErr != nil {
		analysisErr.QuitIf(xerr.ErrorTypeError)
	}

	tl.Log(
		tl.Notice1, palette.GreenBold, "%s",
		"Receipt analysis generated successfully",
	)
	// Log the structured analysis as JSON for inspection.
	tl.LogJSON(tl.Info, palette.Cyan, "ReceiptAnalysis", receiptAnalysis)
}
