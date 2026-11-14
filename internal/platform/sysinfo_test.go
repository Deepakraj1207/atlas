package platform

import (
	"runtime"
	"testing"
)

func TestReadSystemInfoIsHonest(t *testing.T) {
	s := ReadSystemInfo()
	if s.OS == "" {
		t.Error("an OS name is always known")
	}
	if s.MemAvailable > s.MemTotal {
		t.Errorf("available memory exceeds total: %+v", s)
	}
	if s.DiskFree > s.DiskTotal {
		t.Errorf("free disk exceeds total: %+v", s)
	}
	if runtime.GOOS == "linux" && (s.MemTotal == 0 || s.DiskTotal == 0) {
		t.Errorf("Linux should report memory and disk: %+v", s)
	}
}

func TestGBAndMemUsed(t *testing.T) {
	for b, want := range map[uint64]string{17_000_000_000: "15.8 GB", 1 << 30: "1.0 GB", 1_022_000_000_000: "952 GB"} {
		if got := GB(b); got != want {
			t.Errorf("GB(%d) = %q, want %q", b, got, want)
		}
	}
	if (SystemInfo{MemTotal: 10, MemAvailable: 4}).MemUsed() != 6 || (SystemInfo{}).MemUsed() != 0 || (SystemInfo{MemTotal: 1, MemAvailable: 5}).MemUsed() != 0 {
		t.Error("MemUsed")
	}
}
