// you can add any code you want here but don't commit it.
// keep it empty for future projects and for use ase a template.
package main

import (
	"flag"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/otiai10/gosseract/v2"
	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"

	"this-project/src/pkg/config"
)

/*
main is the entrypoint for this example program.

It initializes configuration, parses flags, and runs the image processing
and OCR flow. If any fatal error occurs, it will be logged and the program
will exit with a non-zero status code.
*/
func main() {
	config.CheckIfEnvVarsPresent()

	// Common flags.
	configPath := flag.String("config", "./cfg/config.json", "Path to your configuration file.")

	// Program-specific flags.
	imagePath := flag.String("image", "", "Path to the receipt image to process.")
	outputDirPath := flag.String("out", "./out", "Directory where processed images and OCR text will be stored.")

	// Parse and initialize config.
	flag.Parse()
	config.InitializeConfig(*configPath)

	// Log basic startup information.
	tl.Log(
		tl.Notice, palette.BlueBold, "%s example entrypoint. Config path: '%s'",
		"Running", *configPath,
	)

	// Run the main processing flow.
	runImageProcessingFlow(*imagePath, *outputDirPath).QuitIf(xerr.ErrorTypeError)
}

/*
runImageProcessingFlow orchestrates the overall image processing pipeline.

It performs the following steps:
  1. Validates the input image path.
  2. Ensures the output directory exists.
  3. Generates a timestamp-based base name for output files.
  4. Copies the original image into the output directory with a timestamped name.
  5. Creates a processed version of the image (grayscale, resized, sharpened, contrast-adjusted).
  6. Runs OCR (in Spanish) on the processed image using gosseract.
  7. Saves the OCR text into a .txt file next to the images.

If any step fails, it returns a *xerr.Error describing the problem.
*/
func runImageProcessingFlow(imagePath string, outputDirPath string) (e *xerr.Error) {
	if imagePath == "" {
		err := fmt.Errorf("image path flag '-image' is empty")
		e = xerr.NewError(err, "no input image path provided", imagePath)
		tl.Log(
			tl.Important, palette.PurpleBold, "Exiting early: '%s'",
			"no input image (-image) provided",
		)
		return e
	}

	// Normalize and log initial intent.
	normalizedOutputDirPath := strings.TrimSpace(outputDirPath)
	if normalizedOutputDirPath == "" {
		normalizedOutputDirPath = "./out"
	}

	tl.Log(
		tl.Notice, palette.BlueBold, "%s image processing for '%s' into '%s'",
		"Starting", imagePath, normalizedOutputDirPath,
	)

	// Ensure output directory exists.
	e = ensureOutputDirectory(normalizedOutputDirPath)
	if e != nil {
		return e
	}

	// Generate a timestamp-based base name with filename-safe characters only.
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	baseName := fmt.Sprintf("receipt_%s", timestamp)

	// Build all output paths.
	originalExt := strings.ToLower(filepath.Ext(imagePath))
	if originalExt == "" {
		originalExt = ".jpg"
	}

	originalOutPath := filepath.Join(normalizedOutputDirPath, baseName+"_orig"+originalExt)
	processedOutPath := filepath.Join(normalizedOutputDirPath, baseName+"_clean.png")
	ocrOutPath := filepath.Join(normalizedOutputDirPath, baseName+"_ocr.txt")

	// Copy original image to the output directory.
	e = copyOriginalImage(imagePath, originalOutPath)
	if e != nil {
		return e
	}

	// Create a processed version of the image for better OCR.
	e = createProcessedImage(imagePath, processedOutPath)
	if e != nil {
		return e
	}

	// Run OCR on the processed image.
	var ocrText string
	ocrText, e = runOcrOnImage(processedOutPath)
	if e != nil {
		return e
	}

	// Save OCR result into a text file.
	e = saveOcrTextToFile(ocrOutPath, ocrText)
	if e != nil {
		return e
	}

	tl.Log(
		tl.Info1, palette.Green, "Finished processing image '%s'. Original: '%s', processed: '%s', OCR text: '%s'",
		imagePath, originalOutPath, processedOutPath, ocrOutPath,
	)

	return e
}

/*
ensureOutputDirectory creates the target directory (and parents) if needed.

It uses os.MkdirAll and returns a *xerr.Error if creation fails.
*/
func ensureOutputDirectory(outputDirPath string) (e *xerr.Error) {
	err := os.MkdirAll(outputDirPath, 0o755)
	if err != nil {
		e = xerr.NewError(err, "create output directory", outputDirPath)
		return e
	}

	tl.Log(
		tl.Info1, palette.Blue, "Ensured output directory '%s'",
		outputDirPath,
	)

	return e
}

/*
copyOriginalImage copies the input image file into the target path.

It preserves the file contents exactly, only changing the location and name.
Both paths are provided as full filesystem paths. If any file operation fails,
a *xerr.Error is returned.
*/
func copyOriginalImage(sourcePath string, destinationPath string) (e *xerr.Error) {
	sourceFile, openErr := os.Open(sourcePath)
	if openErr != nil {
		e = xerr.NewError(openErr, "open source image for copy", sourcePath)
		return e
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	destinationFile, createErr := os.Create(destinationPath)
	if createErr != nil {
		e = xerr.NewError(createErr, "create destination image file", destinationPath)
		return e
	}
	defer func() {
		_ = destinationFile.Close()
	}()

	_, copyErr := io.Copy(destinationFile, sourceFile)
	if copyErr != nil {
		e = xerr.NewError(copyErr, "copy image file", fmt.Sprintf("from '%s' to '%s'", sourcePath, destinationPath))
		return e
	}

	tl.Log(
		tl.Info1, palette.Green, "Copied original image to '%s'",
		destinationPath,
	)

	return e
}
/*
createProcessedImage reads the source image, applies preprocessing for OCR,
and saves the result to the destination path as a PNG.

The preprocessing steps are:
  - Convert to grayscale.
  - Resize to double height (keeping aspect ratio) for clearer text.
  - Apply a mild sharpening.
  - Strongly increase contrast.
  - Apply a hard threshold to produce a pure black/white image.

If any step fails, it returns a *xerr.Error.
*/
func createProcessedImage(sourcePath string, destinationPath string) (e *xerr.Error) {
	// Log intent to create processed image.
	tl.Log(
		tl.Info1, palette.Blue, "Creating processed image from '%s' into '%s'",
		sourcePath, destinationPath,
	)

	// Open the source image using the imaging library.
	originalImage, openErr := imaging.Open(sourcePath)
	if openErr != nil {
		e = xerr.NewError(openErr, "open source image for processing", sourcePath)
		return
	}

	// Convert to grayscale for more stable OCR.
	grayscaleImage := imaging.Grayscale(originalImage)

	// Resize (double height, preserve aspect ratio) to help OCR with small text.
	bounds := grayscaleImage.Bounds()
	height := bounds.Dy()
	targetHeight := height * 2
	resizedImage := imaging.Resize(grayscaleImage, 0, targetHeight, imaging.Lanczos)

	// Apply a mild sharpening filter to make edges crisper.
	sharpenedImage := imaging.Sharpen(resizedImage, 1.0)

	// Strongly increase contrast so text stands out from the paper.
	highContrastImage := imaging.AdjustContrast(sharpenedImage, 100.0)

	// Apply a hard threshold to get a pure black/white image.
	// This mimics the aggressive binarization that Tesseract's
	// ImageMagick pipeline tends to like for receipts.
	thresholdValue := uint8(200) // tweak between ~180–220 if needed
	binarizedImage := imaging.AdjustFunc(highContrastImage, func(c color.NRGBA) color.NRGBA {
		// Image is already grayscale, so the red channel is enough
		// as a brightness proxy.
		var brightness uint8 = c.R
		if brightness > thresholdValue {
			return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		}
		return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	})

	// Save the processed image as PNG.
	saveErr := imaging.Save(binarizedImage, destinationPath)
	if saveErr != nil {
		e = xerr.NewError(saveErr, "save processed image", destinationPath)
		return
	}

	tl.Log(
		tl.Info1, palette.Green, "Saved processed image to '%s'",
		destinationPath,
	)

	return
}

/*
runOcrOnImage performs OCR on the given image path using gosseract.

The OCR is configured for Spanish ("spa") and uses the processed image to
extract text. It returns the raw OCR text or a *xerr.Error if something
goes wrong (for example, Tesseract missing, language data missing, or a
read failure).
*/
func runOcrOnImage(imagePath string) (ocrText string, e *xerr.Error) {
	tl.Log(tl.Info1, palette.Cyan, "Running OCR on processed image '%s'", imagePath)

	client := gosseract.NewClient()
	defer func() {
		_ = client.Close()
	}()

	err := client.SetLanguage("spa")
	if err != nil {
		return "", xerr.NewError(err, "unable to client.SetLanguage(\"spa\")", imagePath)
	}
	err = client.SetImage(imagePath)
	if err != nil {
		return "", xerr.NewError(err, "unable to client.SetImage(imagePath)", imagePath)
	}

	ocrText, ocrErr := client.Text()
	if ocrErr != nil {
		return "", xerr.NewError(ocrErr, "unable to run OCR on image", imagePath)
	}

	tl.Log(
		tl.Info1, palette.Cyan, "OCR completed for '%s' (text length: %s)",
		imagePath, fmt.Sprintf("%d", len(ocrText)),
	)

	return ocrText, e
}

/*
saveOcrTextToFile writes the OCR text into a .txt file at the given path.

It overwrites any existing file at that location. If writing fails, it
returns a *xerr.Error.
*/
func saveOcrTextToFile(destinationPath string, ocrText string) (e *xerr.Error) {
	writeErr := os.WriteFile(destinationPath, []byte(ocrText), 0o644)
	if writeErr != nil {
		e = xerr.NewError(writeErr, "write OCR text file", destinationPath)
		return e
	}

	tl.Log(
		tl.Info1, palette.Green, "Saved OCR text to '%s'",
		destinationPath,
	)

	return e
}
