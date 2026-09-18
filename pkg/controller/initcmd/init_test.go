package initcmd_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suzuki-shunsuke/wt/pkg/controller/initcmd"
)

// newLogger returns a logger writing into buf so tests can assert what the user
// is told.
func newLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestController_Init(t *testing.T) { //nolint:funlen,gocognit,cyclop // The length and the branching are the table of cases.
	t.Parallel()
	tests := []struct {
		name string
		// path is joined onto the test's temporary directory, so a nested one also
		// covers creating the parent directories.
		path string
		// existing, when set, is written to path before Init runs.
		existing        string
		wantLogContains string
	}{
		{
			name:            "the file is created",
			path:            "wt.yaml",
			wantLogContains: "has been created",
		},
		{
			name:            "the parent directories are created",
			path:            filepath.Join("wt", "wt.yaml"),
			wantLogContains: "has been created",
		},
		{
			name:            "an existing file is kept",
			path:            "wt.yaml",
			existing:        "root: /somewhere/else\n",
			wantLogContains: "already exists",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), tt.path)
			if tt.existing != "" {
				if err := os.WriteFile(path, []byte(tt.existing), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			buf := &bytes.Buffer{}
			if err := initcmd.New().Init(newLogger(buf), path); err != nil {
				t.Fatalf("Init() error: %v", err)
			}
			if !strings.Contains(buf.String(), tt.wantLogContains) {
				t.Errorf("log does not contain %q:\n%s", tt.wantLogContains, buf)
			}

			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read the file: %v", err)
			}
			if tt.existing != "" {
				if string(content) != tt.existing {
					t.Errorf("the existing file was overwritten: %q", content)
				}
				return
			}
			// The written file has to be a configuration wt can actually load, so it
			// must carry the one setting it needs.
			if !strings.Contains(string(content), "root:") {
				t.Errorf("the created file does not set root:\n%s", content)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			// The file is created with 0644, but the process umask may clear bits from
			// it. What matters is that nothing beyond 0644 is granted.
			if perm := info.Mode().Perm(); perm|0o644 != 0o644 {
				t.Errorf("file permissions = %o, want no bits beyond 644", perm)
			}
		})
	}
}
