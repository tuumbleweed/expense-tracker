package config

import (
	"encoding/json"
	"os"

	"github.com/tuumbleweed/xerr"
	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
)

/*
Load a config file located at filePath
Pass a pointer to whatever config variable you need.
Make sure that file matches the config type.

It's ok to use logger before loading it's config. It still can print but won't save log line to a file

logLevelOverride is used to print all messages (by default log level is 50 so only first message is printed)
pass -1 if you don't want to change logging level from it's default value
need this since config has no effect yet during LoadConfig.

We keep it in util package in order to reuse it in case of projects with multiple config packages
*/
func LoadConfig(filePath string, config any) (e *xerr.Error) {
	tl.Log(tl.Notice, palette.Blue, "%s config '%s'", "Loading", filePath)
	byteValue, err := os.ReadFile(filePath)
	if err != nil {
		return xerr.NewErrorECOL(err, "Unable to read a file", "file path", filePath)
	}

	err = json.Unmarshal(byteValue, &config)
	if err != nil {
		return xerr.NewErrorECOL(err, "Unable to json.Unmarshl file contents to config", "file contents", string(filePath))
	}

	tl.Log(tl.Notice1, palette.Green, "%s config '%s'", "Loaded", filePath)
	return nil
}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}
