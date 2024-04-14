package flagutil

import (
	"regexp"
	"strings"

	"github.com/urfave/cli/v2"
)

var unsafeFlagName = regexp.MustCompile(`[^a-zA-Z0-9_]`)
var dedupUnder = regexp.MustCompile(`__+`)

func computeEnvVar(name string) string {
	return strings.ToUpper(
		dedupUnder.ReplaceAllString(
			unsafeFlagName.ReplaceAllString(name, "_"),
			"_"))
}

func String(dest *string, longName string, alias []string, usage string, required bool) *cli.StringFlag {
	return &cli.StringFlag{
		Destination: dest,
		Value:       *dest,
		Name:        longName,
		Aliases:     alias,
		Required:    required,
		EnvVars:     []string{computeEnvVar(longName)},
	}
}
