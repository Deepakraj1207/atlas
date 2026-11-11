//go:build windows

package platform

import "os"

// stateBaseDir returns %LocalAppData%, the conventional home for per-user
// machine-local data on Windows.
func stateBaseDir() (string, error) {
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		return dir, nil
	}
	// os.UserCacheDir also resolves to %LocalAppData% on Windows.
	return os.UserCacheDir()
}
