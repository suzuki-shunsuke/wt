// Package config resolves and loads wt's configuration file.
//
// The configuration holds the single setting wt needs: the directory under which
// repositories live. wt derives every path it touches from it, so there is no
// implicit default; a missing or empty root is an error pointing at 'wt init'
// rather than a guess at where the user keeps their repositories.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/suzuki-shunsuke/slog-error/slogerr"
	"gopkg.in/yaml.v3"
)

// Config is the content of the configuration file.
type Config struct {
	// Root is the directory under which repositories live. A repository is at
	// <root>/github.com/<owner>/<repo>, and its worktrees at
	// <root>/github.com/<owner>/<repo>+worktrees/<branch>. A leading ~/ is expanded
	// by Load, so the file stays portable between machines.
	Root string `yaml:"root"`
}

// ErrNotFound is returned by Load when the configuration file does not exist.
// The commands turn it into a message pointing at 'wt init'; it is a sentinel so
// that they can tell it apart from a file that exists but cannot be read.
var ErrNotFound = errors.New("the configuration file is not found. Run `wt init` to create it")

// ErrNoRoot is returned by Load when the configuration file exists but sets no
// root. Without it wt has no idea where to put a worktree.
var ErrNoRoot = errors.New("`root` is not set in the configuration file")

// ResolvePath returns the configuration file path, honoring the --config flag
// first, then the WT_CONFIG environment variable, then the default location.
//
// The default is $XDG_CONFIG_HOME/wt/wt.yaml, falling back to ~/.config/wt/wt.yaml
// when XDG_CONFIG_HOME is unset, which is where the rest of the user's tools keep
// their configuration.
//
// getenv is injected so the resolution can be driven by a test without touching
// the process environment.
func ResolvePath(flagPath string, getenv func(string) string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if p := getenv("WT_CONFIG"); p != "" {
		return p, nil
	}
	if dir := getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "wt", "wt.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get the user home directory: %w", err)
	}
	return filepath.Join(home, ".config", "wt", "wt.yaml"), nil
}

// Load reads the configuration file at path.
//
// It returns ErrNotFound when the file does not exist and ErrNoRoot when it sets
// no root, so the caller can point the user at 'wt init' in either case. A root
// starting with ~/ is expanded here rather than at every use, so the rest of wt
// only ever sees an absolute path.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			//nolint:wrapcheck // The error is this package's own sentinel, only annotated.
			return nil, slogerr.With(ErrNotFound, "config_path", path)
		}
		return nil, fmt.Errorf("read the configuration file: %w", slogerr.With(err, "config_path", path))
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("parse the configuration file as YAML: %w", slogerr.With(err, "config_path", path))
	}
	if cfg.Root == "" {
		//nolint:wrapcheck // The error is this package's own sentinel, only annotated.
		return nil, slogerr.With(ErrNoRoot, "config_path", path)
	}
	root, err := expandHome(cfg.Root)
	if err != nil {
		return nil, err
	}
	cfg.Root = root
	return cfg, nil
}

// expandHome replaces a leading ~/ with the user's home directory. A bare "~" is
// expanded too. Any other path is returned unchanged, including one holding a ~
// somewhere other than the front, which is a legal directory name.
func expandHome(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~"+string(filepath.Separator)) {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get the user home directory: %w", err)
	}
	if p == "~" {
		return home, nil
	}
	return filepath.Join(home, p[2:]), nil
}
