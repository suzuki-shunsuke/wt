package initcmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// Init writes the default configuration file to path, creating the parent
// directories as needed.
//
// An existing file is kept as it is and reported rather than treated as an
// error: running 'wt init' on a configuration that is already there is how a
// user checks that they have one, and failing would make the command unsafe to
// re-run.
func (c *Controller) Init(logger *slog.Logger, path string) error {
	if _, err := os.Stat(path); err == nil {
		logger.Info("the configuration file already exists", "config_path", path)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check if a configuration file exists: %w", err)
	}

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:mnd // The standard mode for a directory.
			return fmt.Errorf("create the configuration file directory: %w", err)
		}
	}
	// 0644: the configuration holds no secret, and the user's other tools read
	// their configuration from files with the same mode.
	if err := os.WriteFile(path, configTemplate, 0o644); err != nil { //nolint:gosec,mnd // See above.
		return fmt.Errorf("create the configuration file: %w", err)
	}
	logger.Info("the configuration file has been created. Edit `root` if your repositories live elsewhere",
		"config_path", path)
	return nil
}
