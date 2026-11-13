//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsFolders(t *testing.T) {
	t.Setenv(EnvHome, "")
	data, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" && !strings.EqualFold(filepath.Dir(data), local) {
		t.Errorf("data folder %q should be under %%LocalAppData%% (%q)", data, local)
	}
	logs, _ := LogDir()
	if !strings.HasSuffix(strings.ToLower(logs), `atlas\logs`) {
		t.Errorf("log folder = %q", logs)
	}
	cfg, _ := ConfigDir()
	if appdata := os.Getenv("APPDATA"); appdata != "" && !strings.EqualFold(filepath.Dir(cfg), appdata) {
		t.Errorf("settings folder %q should be under %%AppData%% (%q)", cfg, appdata)
	}
	t.Logf("administrator: %v", IsElevated())
}
