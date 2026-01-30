package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
main runs the full receipt pipeline.

-input / -image can be:
  - a single image file (.jpg/.jpeg/.png)
  - a directory containing images (.jpg/.jpeg/.png)

For each image:
  1) OCR into an output run directory
  2) Run LLM receipt analysis using OCR text + image
  3) Save receipt-analysis.json into the same run directory
*/
func main() {
	config.CheckIfEnvVarsPresent("OPENAI_API_KEY")

	// Common flags.
	configPath := flag.String("config", "./cfg/config.json", "Path to your configuration file.")

	// Program-specific flags.
	imagePath := flag.String("image", "", "Path to a receipt image OR a directory with images (.jpg/.jpeg/.png).")
	outputDirPath := flag.String("out", "./out", "Directory where processed images and OCR text will be stored.")
	language := flag.String("language", "eng+spa", "Language of the receipt. eng, spa, por, spa+eng etc. \"tesseract --list-langs\", \"apt install tesseract-ocr-fra\"")

	flag.Parse()
	util.RequiredFlag(imagePath, "image")
	util.EnsureFlags()
	config.InitializeConfig(*configPath)

	// Build year-month suffix like "september-2006".
	currentTime := time.Now()
	monthName := strings.ToLower(currentTime.Month().String())
	yearValue := currentTime.Year()
	yearMonthDirName := fmt.Sprintf("%s-%04d", monthName, yearValue)

	finalOutputDirPath := filepath.Join(*outputDirPath, yearMonthDirName)

	tl.Log(
		tl.Notice, palette.BlueBold, "%s entrypoint. Config path: '%s'",
		"Running full receipt pipeline", *configPath,
	)
	tl.Log(
		tl.Info1, palette.Cyan, "%s '%s'",
		"Using output directory", finalOutputDirPath,
	)

	imagesToProcess, e := resolveImagesToProcess(*imagePath)
	e.QuitIf("error")

	if len(imagesToProcess) == 0 {
		tl.Log(
			tl.Warning, palette.PurpleBold, "No .jpg/.jpeg/.png files found at: '%s'",
			*imagePath,
		)
		os.Exit(0)
	}

	if len(imagesToProcess) > 1 {
		tl.Log(
			tl.Notice1, palette.GreenBold, "Found '%d' images to process",
			len(imagesToProcess),
		)
	}

	processedCount := 0
	skippedCount := 0

	for _, imgPath := range imagesToProcess {
		tl.Log(tl.Notice, palette.BlueBold, "%s '%s'", "Processing image", imgPath)

		runDirPath, e := processOneImage(imgPath, finalOutputDirPath, *language)
		if e != nil {
			skippedCount++
			tl.Log(
				tl.Error, palette.RedBold, "Failed processing '%s': '%s'",
				imgPath, e,
			)
			continue
		}

		processedCount++
		tl.Log(
			tl.Notice1, palette.GreenBold, "%s. Results stored in '%s'",
			"OCR+analysis completed", runDirPath,
		)
	}

	tl.Log(
		tl.Notice, palette.GreenBold, "Done. Processed: '%s', skipped: '%s'",
		processedCount, skippedCount,
	)
}

func resolveImagesToProcess(inputPath string) (images []string, e *xerr.Error) {
	trimmed := strings.TrimSpace(inputPath)
	if trimmed == "" {
		err := fmt.Errorf("input path is empty")
		e = xerr.NewError(err, "missing -image input", inputPath)
		return
	}

	info, statErr := os.Stat(trimmed)
	if statErr != nil {
		e = xerr.NewError(statErr, "stat -image input path", trimmed)
		return
	}

	if info.IsDir() {
		return listImagesInDir(trimmed)
	}

	// File path
	ext := strings.ToLower(filepath.Ext(trimmed))
	if !isAllowedImageExt(ext) {
		err := fmt.Errorf("unsupported image extension: %s", ext)
		e = xerr.NewError(err, "input file is not .jpg/.jpeg/.png", trimmed)
		return
	}

	return []string{trimmed}, nil
}

func listImagesInDir(dirPath string) (images []string, e *xerr.Error) {
	entries, readErr := os.ReadDir(dirPath)
	if readErr != nil {
		e = xerr.NewError(readErr, "read directory", dirPath)
		return
	}

	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}

		nameLower := strings.ToLower(ent.Name())
		ext := strings.ToLower(filepath.Ext(nameLower))
		if !isAllowedImageExt(ext) {
			continue
		}

		images = append(images, filepath.Join(dirPath, ent.Name()))
	}

	sort.Strings(images)
	return
}

func isAllowedImageExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png":
		return true
	default:
		return false
	}
}

func processOneImage(imagePath string, finalOutputDirPath string, language string) (runDirPath string, e *xerr.Error) {
	// 1) OCR pipeline
	runDirPath, e = ocr.ProcessImage(imagePath, finalOutputDirPath, language)
	if e != nil {
		return "", e
	}

	// 2) Load OCR outputs for analysis
	pricesPath := filepath.Join(runDirPath, "prices.json")
	ocrTextPath := filepath.Join(runDirPath, "ocr.txt")

	ocrTextBytes, readErr := os.ReadFile(ocrTextPath)
	if readErr != nil {
		e = xerr.NewError(readErr, "read OCR text file", ocrTextPath)
		return "", e
	}
	ocrText := string(ocrTextBytes)

	ocrPrices, e := llm.ReadOcrPricesFromFile(pricesPath)
	if e != nil {
		return "", e
	}

	origImagePath, e := findOriginalImagePath(runDirPath)
	if e != nil {
		return "", e
	}

	tl.Log(
		tl.Info1, palette.Cyan, "Loaded OCR artifacts from '%s' (ocr len: '%s', image: '%s')",
		runDirPath, fmt.Sprintf("%d", len(ocrText)), origImagePath,
	)

	// 3) LLM analysis
	receiptAnalysis, analysisErr := llm.GenerateReceiptAnalysisFromImage(origImagePath, ocrText, ocrPrices, nil)
	if analysisErr != nil {
		return "", analysisErr
	}

	// In batch mode, don’t kill the whole run; just skip this image.
	if receiptAnalysis.Totals.TotalCheckMessage != "" {
		tl.Log(
			tl.Warning, palette.PurpleBold, "Receipt total does not match sum of items: '%s'",
			receiptAnalysis.Totals.TotalCheckMessage,
		)
		tl.Log(tl.Warning1, palette.PurpleBold, "%s", "Try taking a photo again")
		err := fmt.Errorf("totals mismatch")
		e = xerr.NewError(err, "receipt totals mismatch", runDirPath)
		return "", e
	}

	analysisPath := filepath.Join(runDirPath, "receipt-analysis.json")

	jsonBytes, marshalErr := json.MarshalIndent(receiptAnalysis, "", "  ")
	if marshalErr != nil {
		e = xerr.NewError(marshalErr, "marshal receipt analysis to JSON", runDirPath)
		return "", e
	}

	writeErr := os.WriteFile(analysisPath, jsonBytes, 0o644)
	if writeErr != nil {
		e = xerr.NewError(writeErr, "write receipt-analysis.json file", analysisPath)
		return "", e
	}

	tl.LogJSON(tl.Verbose, palette.CyanDim, "ReceiptAnalysis", receiptAnalysis)

	tl.Log(
		tl.Notice, palette.GreenBold, "%s",
		"Receipt analysis generated and saved successfully",
	)
	tl.Log(
		tl.Info, palette.Green, "%s to '%s'",
		"Saved receipt analysis", analysisPath,
	)

	return runDirPath, nil
}

func findOriginalImagePath(runDirPath string) (imagePath string, e *xerr.Error) {
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

	imagePath = matches[0]
	return
}
