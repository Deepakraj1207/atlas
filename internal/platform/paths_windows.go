//go:build windows

package platform

import (
	"os"

	"golang.org/x/sys/windows"
)

// stateBaseDir returns %LocalAppData%, the conventional home for per-user
// machine-local data on Windows.
func stateBaseDir() (string, error) {
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		return dir, nil
	}
	// os.UserCacheDir also resolves to %LocalAppData% on Windows.
	return os.UserCacheDir()
}

// dataBaseDir is %LocalAppData% on Windows.
func dataBaseDir() (string, error) { return stateBaseDir() }

func isElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
