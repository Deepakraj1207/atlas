//go:build windows

package platform

import (
	"os"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func readSystemInfo() SystemInfo {
	s := SystemInfo{OS: windowsName()}
	if total, avail, ok := globalMemory(); ok {
		s.MemTotal, s.MemAvailable = total, avail
	}
	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	root := drive + `\`
	if p, err := windows.UTF16PtrFromString(root); err == nil {
		var free, total, totalFree uint64
		if windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree) == nil {
			s.DiskTotal, s.DiskFree, s.DiskPath = total, free, drive
		}
	}
	return s
}

// windowsName reads the product name and version from the registry.
// Windows 11 still reports "Windows 10" in ProductName, so the build number
// (22000 and later is Windows 11) decides.
func windowsName() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "Windows"
	}
	defer k.Close()
	name, _, _ := k.GetStringValue("ProductName")
	build, _, _ := k.GetStringValue("CurrentBuild")
	display, _, _ := k.GetStringValue("DisplayVersion")
	return WindowsName(name, build, display)
}

// WindowsName builds a readable name from the registry values.
func WindowsName(product, build, display string) string {
	name := strings.TrimSpace(product)
	if name == "" {
		name = "Windows"
	}
	if n, err := strconv.Atoi(build); err == nil && n >= 22000 {
		name = strings.Replace(name, "Windows 10", "Windows 11", 1)
	}
	if display = strings.TrimSpace(display); display != "" {
		name += " " + display
	}
	return name
}

// memoryStatusEx is MEMORYSTATUSEX from the Windows API.
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var procGlobalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

func globalMemory() (total, avail uint64, ok bool) {
	if procGlobalMemoryStatusEx.Find() != nil {
		return 0, 0, false
	}
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m)))
	if r == 0 {
		return 0, 0, false
	}
	return m.TotalPhys, m.AvailPhys, true
}
