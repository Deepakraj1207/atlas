//go:build windows

package platform

import "testing"

func TestWindowsName(t *testing.T) {
	for _, c := range [][4]string{
		{"Windows 10 Home", "26300", "25H2", "Windows 11 Home 25H2"},
		{"Windows 10 Pro", "19045", "22H2", "Windows 10 Pro 22H2"},
		{"Windows Server 2025 Datacenter", "26100", "24H2", "Windows Server 2025 Datacenter 24H2"},
		{"", "", "", "Windows"},
	} {
		if got := WindowsName(c[0], c[1], c[2]); got != c[3] {
			t.Errorf("WindowsName(%q, %q, %q) = %q, want %q", c[0], c[1], c[2], got, c[3])
		}
	}
}
