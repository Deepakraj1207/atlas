//go:build !linux && !windows

package platform

import "runtime"

func readSystemInfo() SystemInfo { return SystemInfo{OS: runtime.GOOS} }
