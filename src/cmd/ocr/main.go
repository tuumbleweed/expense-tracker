// you can add any code you want here but don't commit it.
// keep it empty for future projects and for use ase a template.
package main

import (
	"flag"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"

	"this-project/src/pkg/config"
	"this-project/src/pkg/ocr"
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
	runDirPath, e := ocr.ProcessImage(*imagePath, *outputDirPath)
	e.QuitIf(xerr.ErrorTypeError)

	tl.Log(tl.Notice1, palette.GreenBold, "%s. Results stored in '%s'", "OCR run completed", runDirPath)
}
