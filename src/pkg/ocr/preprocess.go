package ocr

import (
	"image/color"

	"github.com/disintegration/imaging"
	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"
)

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
