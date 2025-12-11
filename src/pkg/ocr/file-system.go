package ocr

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"
)

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

/*
saveJSONToFile marshals the given value to pretty-printed JSON and writes it
to a .json file at the given path.

It accepts slices, structs, maps, or any JSON-marshalable value. It overwrites
any existing file at that location. If marshalling or writing fails, it
returns a *xerr.Error.
*/
func saveJSONToFile(destinationPath string, value any) (e *xerr.Error) {
	jsonBytes, marshalErr := json.MarshalIndent(value, "", "  ")
	if marshalErr != nil {
		e = xerr.NewError(marshalErr, "marshal value to JSON", destinationPath)
		return e
	}

	writeErr := os.WriteFile(destinationPath, jsonBytes, 0o644)
	if writeErr != nil {
		e = xerr.NewError(writeErr, "write JSON file", destinationPath)
		return e
	}

	tl.Log(
		tl.Info1, palette.Green, "Saved JSON data to '%s'",
		destinationPath,
	)

	return e
}
