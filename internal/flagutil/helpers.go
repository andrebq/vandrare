package flagutil

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/urfave/cli/v2"
)

var unsafeFlagName = regexp.MustCompile(`[^a-zA-Z0-9_]`)
var dedupUnder = regexp.MustCompile(`__+`)

func computeEnvVar(envPrefix, name string) []string {
	if envPrefix == "" {
		return nil
	}
	return []string{fmt.Sprintf("%v_%v", envPrefix, strings.ToUpper(
		dedupUnder.ReplaceAllString(
			unsafeFlagName.ReplaceAllString(name, "_"),
			"_")))}
}

func StringSlice(dest *cli.StringSlice, longName string, alias []string, envPrefix string, usage string, required bool) *cli.StringSliceFlag {
	return &cli.StringSliceFlag{
		Name:        longName,
		Aliases:     alias,
		EnvVars:     computeEnvVar(envPrefix, longName),
		Usage:       usage,
		Required:    required,
		Destination: dest,
	}
}

func String(dest *string, longName string, alias []string, envPrefix string, usage string, required bool) *cli.StringFlag {
	return &cli.StringFlag{
		Destination: dest,
		Value:       *dest,
		Name:        longName,
		Aliases:     alias,
		Required:    required,
		EnvVars:     computeEnvVar(envPrefix, longName),
	}
}

func Bool(dest *bool, longName string, alias []string, envPrefix string, usage string, required bool) *cli.BoolFlag {
	return &cli.BoolFlag{
		Destination: dest,
		Value:       *dest,
		Name:        longName,
		Aliases:     alias,
		Required:    required,
		EnvVars:     computeEnvVar(envPrefix, longName),
	}
}
