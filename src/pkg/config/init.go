// this package exists to hold Cfg global var
// can also change default values here for this project
package config

import (
	"os"
	"path/filepath"
	"strings"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
)

type Config struct {
	// parts present in configuration file (some of the parameters are generated during initilization process)
	Logger *tl.Config `json:"logger"`

	// those parametrs are initialized during InitializeConfig()
	CallerProgramName string `json:"caller_program_name,omitempty"`
}

func GetDefaultConfig() Config {
	callerProgramName := GetCallerProgramNamePanicWrapper(5)
	callerProgramName = strings.TrimPrefix(callerProgramName, "this-project/")
	callerProgramName = strings.TrimPrefix(callerProgramName, "project-layout/")
	return Config{CallerProgramName: callerProgramName}
}

func SetEffectiveValues(userConfig Config) Config {
	userConfig.Logger = &tl.Cfg

	return userConfig
}

// this function will quit when encountering error
// make sure to run common.LoadConfig first here since it's the earliest point we overriding log level from
func InitializeConfig(configPath string) {
	userConfig := GetDefaultConfig()

	serviceNameSuffix := os.Getenv("SERVICE_NAME_SUFFIX")
	tl.Log(
		tl.Important, palette.BlueBold, "%s, config path: '%s', caller: '%s', suffix: '%s'",
		"Initializing", configPath, userConfig.CallerProgramName, serviceNameSuffix,
	)
	if userConfig.Logger != nil {
		userConfig.Logger.LogDir = filepath.Join(userConfig.Logger.LogDir, userConfig.CallerProgramName, serviceNameSuffix)
	}

	if configPath == "" {
		tl.Log(tl.Notice, palette.Cyan, "%s path is '', using %s", "Config", "default config")
		userConfig = GetDefaultConfig()
	} else if FileExists(configPath) {
		LoadConfig(configPath, &userConfig).QuitIf("error")
	} else {
		tl.Log(tl.Notice, palette.Yellow, "%s path is '%s' but file %s, %s", "Config", configPath, "does not exist", "exiting...")
		os.Exit(1)
	}
	tl.InitializeConfig(userConfig.Logger)

	userConfig = SetEffectiveValues(userConfig)
	tl.Log(tl.Important1, palette.GreenBold, "%s, config path: '%s', caller: '%s'", "Initialized", configPath, userConfig.CallerProgramName)
	tl.Log(tl.Info, palette.CyanDim, "%s (JSON):\n'''\n%s\n'''", "Effective User Config", userConfig)
}
