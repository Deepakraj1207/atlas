package logging

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepakraj1207/atlas/internal/config"
)

func TestNewWritesJSONLinesAndRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, config.LogWarn)
	log.Info("hidden")
	log.Warn("shown", "section", "home")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d: %q", len(lines), buf.String())
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if rec["msg"] != "shown" || rec["section"] != "home" {
		t.Errorf("unexpected record: %v", rec)
	}
}

func TestSetupCreatesFileAndAppends(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	log, closeFn, err := Setup(dir, config.LogInfo)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("first")
	closeFn()

	log, closeFn, err = Setup(dir, config.LogInfo)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("second")
	closeFn()

	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "\n") != 2 {
		t.Errorf("want 2 appended lines, got %q", data)
	}
}
