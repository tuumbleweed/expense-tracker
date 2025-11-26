package echomw

import (
	"fmt"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"

	"expense-tracker/src/pkg/config"
)

type Config struct {
	Address             string `json:"address,omitempty"`
	Port                int    `json:"port,omitempty"`
	MiddlewareRateLimit int    `json:"middleware_rate_limit,omitempty"`
	MiddlewareBurst     int    `json:"middleware_burst,omitempty"`
}

func DefaultValueConfig() Config {
	return Config{
		Address:             "127.0.0.1",
		Port:                8401,
		MiddlewareRateLimit: 3,
		MiddlewareBurst:     50,
	}
}

// create config with default values before config gets initialized
var Cfg Config = DefaultValueConfig() // this one we use to access config values from anywhere

/*
If local Config is provided - use it. Replace all missing values with default ones.

If not provided - just use defaultConfig.
*/
func InitializeConfig(localConfig *Config) {
	// If not provided - just use defaultConfig
	if localConfig == nil {
		tl.Log(tl.Info, palette.Purple, "%s config is %s, keeping %s", "echo-middleware", "not provided", "default echo-middleware config")
		return
	}

	defaultConfig := DefaultValueConfig() // Default values to replace some values with during config initialization

	// If local Config is provided - use it
	Cfg = *localConfig

	tl.ApplyDefaults(&Cfg, defaultConfig, func(field string, defVal any) {
		tl.Log(
			tl.Info, palette.Purple,
			"%s field is %s in %s configuration. Using default value: %v",
			field, "missing", config.GetPackageName(), tl.PrettyForStderr(defVal),
		)
	})

	tl.Log(tl.Info, palette.Green, "%s config was %s, using %s", "echo-middleware", "provided", "local echo-middleware config")
	tl.LogJSON(tl.Verbose, palette.CyanDim, fmt.Sprintf("%s configuration", config.GetPackageName()), Cfg)
}
