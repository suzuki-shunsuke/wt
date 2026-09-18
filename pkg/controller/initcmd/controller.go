// Package initcmd implements the controller for 'wt init', which writes the
// default configuration file.
//
// It never overwrites an existing file. 'wt init' is the command a user runs when
// they are unsure whether they have a configuration yet, so it has to be safe to
// run again on a configuration that has been edited.
package initcmd

import _ "embed"

// configTemplate is the configuration file 'wt init' writes. It is embedded
// rather than built from a string literal so that it stays a real YAML file that
// can be edited, and linted, on its own.
//
//go:embed init.yaml
var configTemplate []byte

// Controller creates the configuration file.
type Controller struct{}

// New creates a Controller.
func New() *Controller {
	return &Controller{}
}
