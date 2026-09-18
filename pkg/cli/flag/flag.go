// Package flag provides the flags shared by wt's commands.
//
// Each function registers its flag on the given flag set rather than returning a
// definition, because that is how pflag works; the destination pointer is where
// the value lands.
package flag

import (
	"github.com/spf13/pflag"
	"github.com/suzuki-shunsuke/cobra-util/cobrautil"
)

// GlobalFlags holds the flag values registered on the root command.
type GlobalFlags struct {
	LogLevel string
	Config   string
}

// LogLevel registers the flag setting the logging level.
// Supported values are: debug, info, warn, error.
func LogLevel(fs *pflag.FlagSet, dest *string) {
	fs.StringVar(dest, "log-level", "", "Log level (debug, info, warn, error)")
	cobrautil.Envs(fs, "log-level", "WT_LOG_LEVEL")
}

// Config registers the flag specifying the configuration file path.
// Alias: -c
func Config(fs *pflag.FlagSet, dest *string) {
	fs.StringVarP(dest, "config", "c", "", "configuration file path")
	cobrautil.Envs(fs, "config", "WT_CONFIG")
}
