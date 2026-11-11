//go:build !windows

package platform

import (
	"os"
	"path/filepath"
)

// stateBaseDir follows the XDG base directory spec for state data.
func stateBaseDir() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state"), nil
}
