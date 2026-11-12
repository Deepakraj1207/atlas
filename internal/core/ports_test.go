package core

import "testing"

func TestCommandString(t *testing.T) {
	c := Command{Program: "winget", Args: []string{"upgrade", "--id", "Git.Git", "--name", "My App"}}
	if got := c.String(); got != `winget upgrade --id Git.Git --name "My App"` {
		t.Errorf("String = %s", got)
	}
}

func TestCommandInterpret(t *testing.T) {
	c := Command{}
	if ok, _ := c.Interpret(0); !ok {
		t.Error("0 should be success by default")
	}
	if ok, _ := c.Interpret(1); ok {
		t.Error("1 should fail by default")
	}
	choco := Command{OKCodes: []int{0}, RestartCodes: []int{1641, 3010}}
	if ok, restart := choco.Interpret(3010); !ok || !restart {
		t.Error("3010 should be success with restart")
	}
	dnf := Command{OKCodes: []int{0, 100}}
	if ok, _ := dnf.Interpret(100); !ok {
		t.Error("custom OK code should succeed")
	}
}

func TestStatusSavedAndCounts(t *testing.T) {
	if !ScanPartial.Saved() || ScanFailed.Saved() || ScanCancelled.Saved() {
		t.Error("only complete and partial results are saved")
	}
	apps, comps := CountKinds([]SoftwareItem{{Kind: KindApp}, {Kind: KindComponent}, {}})
	if apps != 2 || comps != 1 {
		t.Errorf("CountKinds = %d, %d", apps, comps)
	}
}
