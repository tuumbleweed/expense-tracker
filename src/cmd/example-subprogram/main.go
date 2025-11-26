// in case you need to create an entrypoint with multiple subprograms
package main

import (
	"flag"
	"os"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"

	"expense-tracker/src/pkg/config"
)

func example(subprogram string, flags []string) {
	config.CheckIfEnvVarsPresent()
	// common flags
	subprogramCmd := flag.NewFlagSet(subprogram, flag.ExitOnError)
	configPath := subprogramCmd.String("config", "./cfg/config.json", "Path to your configuration file.")
	// program's custom flags
	// parse and init config
	xerr.QuitIfError(subprogramCmd.Parse(flags), "Unable to subprogramCmd.Parse")
	config.InitializeConfig(*configPath)

	tl.Log(
		tl.Notice, palette.BlueBold, "%s example-subprogram entrypoint. Subprogram: '%s'. Config path: '%s'",
		"Running", subprogram, *configPath,
	)
}

func main() {
	// Check if there are enough arguments
	if len(os.Args) < 2 {
		tl.Log(tl.Error, palette.Red, "Usage: %s", "go run src/cmd/example-subprogram/main.go subprogram_name(for exampe first-example)")
		os.Exit(0)
	}
	subprogram := os.Args[1]
	flags := os.Args[2:]

	// Switch subprogram based on the first argument
	switch subprogram {
	case "first-example":
		example(subprogram, flags)
	default:
		tl.Log(tl.Error, palette.Red, "Unknown subprogram: %s", subprogram)
		os.Exit(0)
	}
}
