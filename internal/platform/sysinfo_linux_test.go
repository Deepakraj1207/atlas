//go:build linux

package platform

import (
	"bufio"
	"strings"
	"testing"
)

func TestParseMeminfo(t *testing.T) {
	total, avail := parseMeminfo(bufio.NewScanner(strings.NewReader("MemTotal:       16303844 kB\nMemFree:  1 kB\nMemAvailable:    8123456 kB\n")))
	if total != 16303844*1024 || avail != 8123456*1024 {
		t.Errorf("total=%d avail=%d", total, avail)
	}
}
