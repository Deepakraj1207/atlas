//go:build linux

package platform

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func readSystemInfo() SystemInfo {
	var s SystemInfo
	if d, err := ReadOSRelease(); err == nil {
		s.OS = d.Name
	}
	if s.OS == "" {
		s.OS = "Linux"
	}
	if f, err := os.Open("/proc/meminfo"); err == nil {
		s.MemTotal, s.MemAvailable = parseMeminfo(bufio.NewScanner(f))
		f.Close()
	}
	var fs syscall.Statfs_t
	if syscall.Statfs("/", &fs) == nil {
		s.DiskTotal = fs.Blocks * uint64(fs.Bsize)
		s.DiskFree = fs.Bavail * uint64(fs.Bsize)
		s.DiskPath = "/"
	}
	return s
}

// parseMeminfo reads MemTotal and MemAvailable (in kB) from /proc/meminfo.
func parseMeminfo(sc *bufio.Scanner) (total, avail uint64) {
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		n, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil {
			continue
		}
		switch f[0] {
		case "MemTotal:":
			total = n * 1024
		case "MemAvailable:":
			avail = n * 1024
		}
	}
	return total, avail
}
