package platform

import "fmt"

// SystemInfo is what the dashboard shows about this computer. Every field
// may be unknown (empty or zero); the UI then leaves it out rather than
// guessing. Reading it never changes anything.
type SystemInfo struct {
	// OS is a readable name such as "Windows 11 Home 24H2" or
	// "Ubuntu 24.04.1 LTS".
	OS string
	// MemTotal and MemAvailable are bytes of physical memory.
	MemTotal, MemAvailable uint64
	// DiskTotal and DiskFree are bytes on the system drive (Windows) or the
	// root file system (Linux). DiskPath names it.
	DiskTotal, DiskFree uint64
	DiskPath            string
}

// MemUsed is the memory in use, or 0 when unknown.
func (s SystemInfo) MemUsed() uint64 {
	if s.MemTotal == 0 || s.MemAvailable > s.MemTotal {
		return 0
	}
	return s.MemTotal - s.MemAvailable
}

// ReadSystemInfo reads what it can about this computer.
func ReadSystemInfo() SystemInfo { return readSystemInfo() }

// GB formats bytes as gigabytes with one decimal under 100 GB ("15.8 GB")
// and none above ("952 GB").
func GB(b uint64) string {
	g := float64(b) / (1 << 30)
	if g >= 100 {
		return fmt.Sprintf("%.0f GB", g)
	}
	return fmt.Sprintf("%.1f GB", g)
}
