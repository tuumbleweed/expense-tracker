package util

import (
	"os"
	"strings"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
)

var RequiredFlags = map[*string]string{}

// RequiredFlag(senderPtr, "--sender"), can also use --ender and sender
func RequiredFlag(flagPointer *string, cliName string) {
	name := normalizeFlagName(cliName)
	RequiredFlags[flagPointer] = name
}
func normalizeFlagName(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "--") {
		return s
	}
	if strings.HasPrefix(s, "-") {
		// single dash → double dash
		return "-" + s
	}
	return "--" + s
}

// Ensure logs every missing required flag and exits(1) if any were missing.
func EnsureFlags() {
	missing := false
	for flagPointer, cliName := range RequiredFlags {
		if flagPointer == nil || strings.TrimSpace(*flagPointer) == "" {
			tl.Log(tl.Warning, palette.YellowBold, "%s parameter is %s", cliName, "required")
			missing = true
		}
	}
	if missing {
		os.Exit(1)
	}
}
