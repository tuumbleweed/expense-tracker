package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"

	"expense-tracker/src/pkg/config"
	"expense-tracker/src/pkg/llm"
	"expense-tracker/src/pkg/ocr"
	"expense-tracker/src/pkg/util"
)

/*
main runs the full receipt pipeline in one program:
  1) OCR the image into an output run directory
  2) Run LLM receipt analysis using OCR text + image
  3) Save receipt-analysis.json into the same run directory
*/
func main() {
	// Ensure required environment variables are present.
	config.CheckIfEnvVarsPresent("OPENAI_API_KEY")

	// Common flags.
	configPath := flag.String("config", "./cfg/config.json", "Path to your configuration file.")

	// Program-specific flags.
	imagePath := flag.String("image", "", "Path to the receipt image to process.")
	outputDirPath := flag.String("out", "./out", "Directory where processed images and OCR text will be stored.")
	language := flag.String("language", "eng+spa", "Language of the receipt. eng, spa, por, spa+eng etc. \"tesseract --list-langs\", \"apt install tesseract-ocr-fra\"")

	// Parse and initialize config.
	flag.Parse()
	util.RequiredFlag(imagePath, "image")
	util.EnsureFlags()
	config.InitializeConfig(*configPath)

	// Build year-month suffix like "september-2006".
	currentTime := time.Now()
	monthName := strings.ToLower(currentTime.Month().String())
	yearValue := currentTime.Year()
	yearMonthDirName := fmt.Sprintf("%s-%04d", monthName, yearValue)

	// Final output directory: base out dir + "month-year".
	finalOutputDirPath := filepath.Join(*outputDirPath, yearMonthDirName)

	tl.Log(
		tl.Notice, palette.BlueBold, "%s entrypoint. Config path: '%s'",
		"Running full receipt pipeline", *configPath,
	)
	tl.Log(
		tl.Info1, palette.Cyan, "%s '%s'",
		"Using output directory", finalOutputDirPath,
	)

	// 1) OCR pipeline
	runDirPath, e := ocr.ProcessImage(*imagePath, finalOutputDirPath, *language)
	e.QuitIf(xerr.ErrorTypeError)

	tl.Log(
		tl.Notice1, palette.GreenBold, "%s. Run dir: '%s'",
		"OCR completed", runDirPath,
	)

	// 2) Load OCR outputs for analysis
	pricesPath := filepath.Join(runDirPath, "prices.json")
	ocrTextPath := filepath.Join(runDirPath, "ocr.txt")

	ocrTextBytes, readErr := os.ReadFile(ocrTextPath)
	xerr.QuitIfError(readErr, "read OCR text file")
	ocrText := string(ocrTextBytes)

	ocrPrices, e := llm.ReadOcrPricesFromFile(pricesPath)
	e.QuitIf("error")

	origImagePath, e := findOriginalImagePath(runDirPath)
	e.QuitIf("error")

	tl.Log(
		tl.Info1, palette.Cyan, "Loaded OCR artifacts from '%s' (ocr len: '%s', image: '%s')",
		runDirPath, fmt.Sprintf("%d", len(ocrText)), origImagePath,
	)

	// 3) LLM analysis
	receiptAnalysis, analysisErr := llm.GenerateReceiptAnalysisFromImage(origImagePath, ocrText, ocrPrices, nil)
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

	analysisPath := filepath.Join(runDirPath, "receipt-analysis.json")

	jsonBytes, marshalErr := json.MarshalIndent(receiptAnalysis, "", "  ")
	xerr.QuitIfError(marshalErr, "marshal receipt analysis to JSON")

	writeErr := os.WriteFile(analysisPath, jsonBytes, 0o644)
	xerr.QuitIfError(writeErr, "write receipt-analysis.json file")

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

func findOriginalImagePath(runDirPath string) (imagePath string, e *xerr.Error) {
	// OCR pipeline writes "orig.<ext>" (ext depends on input).
	pattern := filepath.Join(runDirPath, "orig.*")
	matches, globErr := filepath.Glob(pattern)
	if globErr != nil {
		e = xerr.NewError(globErr, "glob for original image", pattern)
		return
	}
	if len(matches) == 0 {
		err := fmt.Errorf("no original image found")
		e = xerr.NewError(err, "missing original image (expected orig.*)", runDirPath)
		return
	}

	// If multiple match, just pick the first.
	imagePath = matches[0]
	return
}
