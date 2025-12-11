package ocr

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"
)

/*
ProcessImage orchestrates the overall image processing pipeline.

It performs the following steps:
  1. Validates the input image path.
  2. Ensures the root output directory exists.
  3. Creates a per-run directory under the root, named by timestamp.
  4. Copies the original image into that run directory as orig.<ext>.
  5. Creates a processed version of the image in that run directory as clean.png.
  6. Runs OCR (in Spanish) on clean.png using gosseract.
  7. Saves the OCR text into ocr.txt in the same run directory.

If any step fails, it returns a *xerr.Error describing the problem.
*/
func ProcessImage(imagePath string, outputDirPath string) (runDirPath string, e *xerr.Error) {
	e = validateImagePath(imagePath)
    if e != nil {
        return
    }

	// Normalize and log initial intent.
	normalizedOutputDirPath := strings.TrimSpace(outputDirPath)
	if normalizedOutputDirPath == "" {
		normalizedOutputDirPath = "./out"
	}

	tl.Log(
		tl.Notice, palette.BlueBold, "%s image processing for '%s' into root '%s'",
		"Starting", imagePath, normalizedOutputDirPath,
	)

	// Ensure root output directory exists (e.g. ./out).
	e = ensureOutputDirectory(normalizedOutputDirPath)
	if e != nil {
		return "", e
	}

	// Generate a timestamp-based directory name with filename-safe characters only.
	// Example: 2025-11-26_16-35-31
	timestamp := time.Now().Format("2006-01-02_15-04-05")

	// Per-run directory inside the root, e.g. ./out/2025-11-26_16-35-31
	runDirPath = filepath.Join(normalizedOutputDirPath, timestamp)

	// Ensure per-run directory exists.
	e = ensureOutputDirectory(runDirPath)
	if e != nil {
		return runDirPath, e
	}

	// Determine original extension (keep the dot).
	originalExt := strings.ToLower(filepath.Ext(imagePath))
	if originalExt == "" {
		originalExt = ".jpg"
	}

	// Build all output paths inside the per-run directory.
	originalOutPath := filepath.Join(runDirPath, "orig"+originalExt)
	processedOutPath := filepath.Join(runDirPath, "clean.png")
	ocrOutPath := filepath.Join(runDirPath, "ocr.txt")
	ocrNumbersOutPath := filepath.Join(runDirPath, "numbers-ocr.txt")
	pricesPath := filepath.Join(runDirPath, "prices.json")

	// Copy original image to the run directory.
	e = copyOriginalImage(imagePath, originalOutPath)
	if e != nil {
		return runDirPath, e
	}

	// Create a processed version of the image for better OCR.
	e = createProcessedImage(imagePath, processedOutPath)
	if e != nil {
		return runDirPath, e
	}

	// Run OCR on the processed image.
	var numbersOcr, ocrText string
	numbersOcr, e = runOcrForNumbers(processedOutPath)
	if e != nil {
		return runDirPath, e
	}
	ocrText, e = runOcrOnImage(processedOutPath)
	if e != nil {
		return runDirPath, e
	}

	prices := ExtractPriceCandidates(numbersOcr)
	tl.Log(tl.Info, palette.Cyan, "Extracted prices: '%s'", prices)

	// Save OCR result into a text file.
	e = saveOcrTextToFile(ocrNumbersOutPath, numbersOcr)
	if e != nil {
		return runDirPath, e
	}

	// Save OCR result into a text file.
	e = saveOcrTextToFile(ocrOutPath, ocrText)
	if e != nil {
		return runDirPath, e
	}

	// Save OCR result into a text file.
	e = saveJSONToFile(pricesPath, prices)
	if e != nil {
		return runDirPath, e
	}

	tl.Log(
		tl.Info1, palette.Green, "Finished processing image '%s'. Run dir: '%s', original: '%s', processed: '%s', OCR text: '%s'",
		imagePath, runDirPath, originalOutPath, processedOutPath, ocrOutPath,
	)

	return runDirPath, e
}

/*
validateImagePath ensures the image path is not empty and exists.
Right now it just checks for empty input and wraps that into *xerr.Error,
but can be extended to os.Stat, extension checks, etc.
*/
func validateImagePath(imagePath string) (e *xerr.Error) {
    if imagePath == "" {
        err := fmt.Errorf("image path flag '-image' is empty")
        e = xerr.NewError(err, "no input image path provided", imagePath)
        tl.Log(
            tl.Important, palette.PurpleBold, "Exiting early: '%s'",
            "no input image (-image) provided",
        )
    }
    return
}
