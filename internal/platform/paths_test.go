package platform

import (
	"path/filepath"
	"testing"
)

func TestAtlasHomeOverridesBothDirs(t *testing.T) {
	t.Setenv(EnvHome, filepath.Join("some", "portable", "dir"))

	cfg, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	if cfg != filepath.Join("some", "portable", "dir") {
		t.Errorf("ConfigDir = %q, want override", cfg)
	}

	logs, err := LogDir()
	if err != nil {
		t.Fatalf("LogDir: %v", err)
	}
	if logs != filepath.Join("some", "portable", "dir", "logs") {
		t.Errorf("LogDir = %q, want override/logs", logs)
	}
}

func TestDefaultDirsEndWithAppName(t *testing.T) {
	t.Setenv(EnvHome, "")

	cfg, err := ConfigDir()
	if err != nil {
		t.Skipf("no user config dir in this environment: %v", err)
	}
	if filepath.Base(cfg) != appDirName {
		t.Errorf("ConfigDir = %q, want to end with %q", cfg, appDirName)
	}

	logs, err := LogDir()
	if err != nil {
		t.Skipf("no state dir in this environment: %v", err)
	}
	if filepath.Base(logs) != "logs" || filepath.Base(filepath.Dir(logs)) != appDirName {
		t.Errorf("LogDir = %q, want .../%s/logs", logs, appDirName)
	}
}
