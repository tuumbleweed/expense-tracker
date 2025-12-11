package ocr

import (
	"fmt"

	"github.com/otiai10/gosseract/v2"
	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"
)

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

	// 🔹 Preserve multiple spaces between words/columns
	err = client.SetVariable("preserve_interword_spaces", "1")
	if err != nil {
		return "", xerr.NewError(err, "unable to client.SetVariable(\"preserve_interword_spaces\", \"1\")", imagePath)
	}

	// Match CLI: `--psm 6` (single uniform block of text).
	err = client.SetPageSegMode(gosseract.PSM_SINGLE_BLOCK)
	if err != nil {
		e = xerr.NewError(err, "unable to client.SetPageSegMode(PSM_SINGLE_BLOCK)", imagePath)
		return
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
		tl.Info1, palette.Green, "OCR completed for '%s' (text length: %s)",
		imagePath, fmt.Sprintf("%d", len(ocrText)),
	)

	return ocrText, e
}

