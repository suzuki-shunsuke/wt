package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suzuki-shunsuke/wt/pkg/config"
)

func TestResolvePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		flagPath string
		envs     map[string]string
		want     string
		// wantSuffix is checked instead of want when the expected path depends on the
		// home directory, which a test must not hard-code.
		wantSuffix string
	}{
		{
			name:     "the flag wins",
			flagPath: "/flag/wt.yaml",
			envs:     map[string]string{"WT_CONFIG": "/env/wt.yaml", "XDG_CONFIG_HOME": "/xdg"},
			want:     "/flag/wt.yaml",
		},
		{
			name: "WT_CONFIG wins over XDG_CONFIG_HOME",
			envs: map[string]string{"WT_CONFIG": "/env/wt.yaml", "XDG_CONFIG_HOME": "/xdg"},
			want: "/env/wt.yaml",
		},
		{
			name: "XDG_CONFIG_HOME is used",
			envs: map[string]string{"XDG_CONFIG_HOME": "/xdg"},
			want: filepath.Join("/xdg", "wt", "wt.yaml"),
		},
		{
			name:       "the default is under the home directory",
			envs:       map[string]string{},
			wantSuffix: filepath.Join(".config", "wt", "wt.yaml"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := config.ResolvePath(tt.flagPath, func(k string) string { return tt.envs[k] })
			if err != nil {
				t.Fatalf("ResolvePath() error: %v", err)
			}
			if tt.wantSuffix != "" {
				if !strings.HasSuffix(got, tt.wantSuffix) {
					t.Errorf("ResolvePath() = %q, want it to end with %q", got, tt.wantSuffix)
				}
				return
			}
			if got != tt.want {
				t.Errorf("ResolvePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoad(t *testing.T) { //nolint:funlen,gocognit,cyclop // The length and the branching are the table of cases.
	t.Parallel()
	tests := []struct {
		name string
		// content is written to the configuration file. When it is empty the file is
		// not created at all, which is the missing-file case.
		content string
		want    string
		wantErr error
		// wantErrContains is checked when the failure has no sentinel to compare.
		wantErrContains string
		// wantHome asserts that the root was expanded to the home directory.
		wantHome bool
	}{
		{
			name:    "an absolute root is read as it is",
			content: "root: /repos/src\n",
			want:    "/repos/src",
		},
		{
			name:     "a leading tilde is expanded",
			content:  "root: ~/repos/src\n",
			wantHome: true,
		},
		{
			name:    "a missing file reports ErrNotFound",
			wantErr: config.ErrNotFound,
		},
		{
			name:    "an empty root reports ErrNoRoot",
			content: "root: \"\"\n",
			wantErr: config.ErrNoRoot,
		},
		{
			name:    "a file without root reports ErrNoRoot",
			content: "# nothing here\n",
			wantErr: config.ErrNoRoot,
		},
		{
			name:            "invalid YAML is reported",
			content:         "root: [\n",
			wantErrContains: "parse the configuration file as YAML",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "wt.yaml")
			if tt.content != "" {
				if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			cfg, err := config.Load(path)
			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Load() error = %v, want %v", err, tt.wantErr)
				}
				return
			case tt.wantErrContains != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("Load() error = %v, want it to contain %q", err, tt.wantErrContains)
				}
				return
			case err != nil:
				t.Fatalf("Load() error: %v", err)
			}

			if tt.wantHome {
				home, err := os.UserHomeDir()
				if err != nil {
					t.Fatal(err)
				}
				if want := filepath.Join(home, "repos", "src"); cfg.Root != want {
					t.Errorf("Root = %q, want %q", cfg.Root, want)
				}
				return
			}
			if cfg.Root != tt.want {
				t.Errorf("Root = %q, want %q", cfg.Root, tt.want)
			}
		})
	}
}
