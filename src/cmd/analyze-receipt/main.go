// you can add any code you want here but don't commit it.
// keep it empty for future projects and for use ase a template.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
	// Parse flags.
	flag.Parse()
	// Mark required flags and ensure they are present.
	util.RequiredFlag(ocrTextPath, "ocr-text")
	util.EnsureFlags()
	// Initialize configuration.
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
		tl.Info1, palette.Cyan, "Loaded OCR text from '%s' (length: '%s')",
		*ocrTextPath, fmt.Sprintf("%d", len(ocrText)),
	)

	// For now, pass nil to use the default category map inside the LLM layer.
	receiptAnalysis, analysisErr := llm.GenerateReceiptAnalysis(ocrText, nil)
	if analysisErr != nil {
		analysisErr.QuitIf(xerr.ErrorTypeError)
	}

	// If totals do not match, log a warning and stop the program.
	if receiptAnalysis.Totals.TotalCheckMessage != "" {
		tl.Log(
			tl.Warning, palette.PurpleBold, "Receipt total does not match sum of items: '%s'",
			receiptAnalysis.Totals.TotalCheckMessage,
		)
		tl.Log(tl.Warning1, palette.PurpleBold, "%s", "Try taking a photo again")
		os.Exit(0)
	}

	// Totals are OK; proceed to save the analysis JSON next to the OCR text file.
	runDirPath := filepath.Dir(*ocrTextPath)
	analysisPath := filepath.Join(runDirPath, "receipt-analysis.json")

	jsonBytes, marshalErr := json.MarshalIndent(receiptAnalysis, "", "  ")
	xerr.QuitIfError(marshalErr, "marshal receipt analysis to JSON")

	writeErr := os.WriteFile(analysisPath, jsonBytes, 0o644)
	xerr.QuitIfError(writeErr, "write receipt-analysis.json file")

	// Log the structured analysis as JSON for inspection.
	tl.LogJSON(tl.Verbose, palette.CyanDim, "ReceiptAnalysis", receiptAnalysis)

	tl.Log(
		tl.Notice, palette.GreenBold, "%s",
		"Receipt analysis generated and saved successfully",
	)
	tl.Log(
		tl.Info, palette.Green, "%s to '%s'",
		"Saved receipt analysis", analysisPath,
	)
}
