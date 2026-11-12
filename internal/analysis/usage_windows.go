//go:build windows

package analysis

import (
	"context"
	"os"

	"golang.org/x/sys/windows/registry"

	"github.com/deepakraj1207/atlas/internal/core"
)

type userAssist struct{}

// NewUsageSource reads Windows' UserAssist records, which note when
// programs were opened from the Start menu, taskbar or desktop. The data
// stays on this computer.
func NewUsageSource() Source { return userAssist{} }

func (userAssist) ID() string    { return "usage" }
func (userAssist) Label() string { return "When programs were last opened (Windows)" }
func (userAssist) Detail() string {
	return `Reads HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\UserAssist (read-only)`
}

func (userAssist) Read(ctx context.Context) (Input, error) {
	const path = `Software\Microsoft\Windows\CurrentVersion\Explorer\UserAssist`
	root, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return Input{}, core.ErrSourceUnavailable
	}
	defer root.Close()
	guids, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return Input{}, err
	}
	in := Input{IsUsage: true}
	for _, g := range guids {
		if err := ctx.Err(); err != nil {
			return Input{}, err
		}
		k, err := registry.OpenKey(root, g+`\Count`, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		names, _ := k.ReadValueNames(-1)
		for _, n := range names {
			data, _, err := k.GetBinaryValue(n)
			if err != nil {
				continue
			}
			count, last, ok := parseUserAssistValue(data)
			if !ok {
				continue
			}
			in.Usage = append(in.Usage, UsageRecord{Path: resolveUserAssistPath(rot13(n), os.Getenv), RunCount: count, LastRun: last})
		}
		k.Close()
	}
	return in, nil
}
