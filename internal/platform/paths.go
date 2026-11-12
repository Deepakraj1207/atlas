// Package platform isolates operating-system specific behaviour. Everything
// else in ATLAS should go through this package rather than checking
// runtime.GOOS directly.
package platform

import (
	"os"
	"path/filepath"
)

const appDirName = "atlas"

// EnvHome is an environment variable that, when set, overrides both the
// configuration and log directories. Useful for portable installs and tests.
const EnvHome = "ATLAS_HOME"

// ConfigDir returns the directory that holds config.yaml.
//
//	Windows: %AppData%\atlas
//	Linux:   $XDG_CONFIG_HOME/atlas or ~/.config/atlas
func ConfigDir() (string, error) {
	if home := os.Getenv(EnvHome); home != "" {
		return home, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName), nil
}

// LogDir returns the directory for log files.
//
//	Windows: %LocalAppData%\atlas\logs
//	Linux:   $XDG_STATE_HOME/atlas/logs or ~/.local/state/atlas/logs
func LogDir() (string, error) {
	if home := os.Getenv(EnvHome); home != "" {
		return filepath.Join(home, "logs"), nil
	}
	base, err := stateBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName, "logs"), nil
}

// DataDir returns the directory for ATLAS's database.
//
//	Windows: %LocalAppData%\atlas
//	Linux:   $XDG_DATA_HOME/atlas or ~/.local/share/atlas
func DataDir() (string, error) {
	if home := os.Getenv(EnvHome); home != "" {
		return home, nil
	}
	base, err := dataBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName), nil
}

// IsElevated reports whether ATLAS is running with administrator (Windows)
// or root (Unix) rights. It is used only to explain and plan updates; ATLAS
// never requests elevation for itself.
func IsElevated() bool { return isElevated() }
