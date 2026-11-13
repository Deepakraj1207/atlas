package handover

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

func TestJobRunsStepsAndReportsResults(t *testing.T) {
	var calls []string
	start := func(c core.Command, in io.Reader, out, errw io.Writer) (int, error) {
		calls = append(calls, c.String())
		switch c.Program {
		case "bad":
			return 1, nil
		case "missing":
			return -1, errors.New("missing isn't installed or isn't on PATH")
		case "reboot":
			return 3010, nil
		}
		return 0, nil
	}
	steps := []core.UpdateStep{
		{Label: "Good", Updates: []core.Update{{Name: "A", CurrentVersion: "1", AvailableVersion: "2"}}, Commands: []core.Command{{Program: "ok", Args: []string{"one"}}, {Program: "ok", Args: []string{"two"}}}, Elevation: core.ElevationSudo},
		{Label: "Blocked", Updates: []core.Update{{Name: "B"}}, Blocked: "needs admin"},
		{Label: "Bad", Updates: []core.Update{{Name: "C"}}, Commands: []core.Command{{Program: "bad"}, {Program: "never"}}},
		{Label: "Missing", Updates: []core.Update{{Name: "D"}}, Commands: []core.Command{{Program: "missing"}}},
		{Label: "Reboot", Updates: []core.Update{{Name: "E"}}, Commands: []core.Command{{Program: "reboot", RestartCodes: []int{3010}}}},
	}
	j := New(steps, start, func() (<-chan os.Signal, func()) { return make(chan os.Signal), func() {} })
	var out bytes.Buffer
	j.SetStdin(strings.NewReader("\n"))
	j.SetStdout(&out)
	j.SetStderr(&out)
	if err := j.Run(); err != nil {
		t.Fatal(err)
	}
	r := j.Results()
	want := []core.StepStatus{core.StepDone, core.StepSkipped, core.StepFailed, core.StepFailed, core.StepDone}
	for i, s := range want {
		if r[i].Status != s {
			t.Errorf("step %d: %s, want %s", i, r[i].Status, s)
		}
	}
	if strings.Join(calls, ";") != "ok one;ok two;bad;missing;reboot" {
		t.Errorf("calls = %v (a failed command must stop its step)", calls)
	}
	if r[2].ExitCode != 1 || !strings.Contains(r[3].Note, "isn't installed") || !r[4].Restart {
		t.Errorf("results = %+v", r)
	}
	text := out.String()
	for _, w := range []string{"ATLAS never answers for you", "[1/4] Good", "$ ok one", "sudo may ask for your password", "Skipped B: needs admin", "Didn't work (exit code 1)", "restart is needed", "Finished: 2 updated, 2 didn't work, 1 skipped.", "Press Enter to return to ATLAS"} {
		if !strings.Contains(text, w) {
			t.Errorf("terminal output missing %q:\n%s", w, text)
		}
	}
}

func TestJobStopsAfterInterrupt(t *testing.T) {
	sig := make(chan os.Signal, 1)
	start := func(c core.Command, in io.Reader, out, errw io.Writer) (int, error) {
		sig <- os.Interrupt               // user presses Ctrl+C while this runs
		time.Sleep(20 * time.Millisecond) // the package manager exits
		return 130, nil
	}
	j := New([]core.UpdateStep{
		{Label: "First", Updates: []core.Update{{Name: "A"}}, Commands: []core.Command{{Program: "x"}}},
		{Label: "Second", Updates: []core.Update{{Name: "B"}}, Commands: []core.Command{{Program: "y"}}},
	}, start, func() (<-chan os.Signal, func()) { return sig, func() {} })
	j.SetStdin(strings.NewReader("\n"))
	j.SetStdout(io.Discard)
	j.Run()
	r := j.Results()
	if r[0].Status != core.StepStopped || r[1].Status != core.StepStopped || !strings.Contains(r[1].Note, "Not started") {
		t.Errorf("results = %+v", r)
	}
}

func TestVerbsFollowTheStepAction(t *testing.T) {
	start := func(c core.Command, in io.Reader, out, errw io.Writer) (int, error) { return 0, nil }
	var out bytes.Buffer
	j := New([]core.UpdateStep{
		{Label: "APT", Action: core.ActionRemove, Updates: []core.Update{{Name: "meld", CurrentVersion: "3.22"}}, Commands: []core.Command{{Program: "x"}}},
		{Label: "Windows Package Manager", Action: core.ActionRemove, Elevation: core.ElevationInstaller, Updates: []core.Update{{Name: "jq"}}, Commands: []core.Command{{Program: "x"}}},
	}, start, func() (<-chan os.Signal, func()) { return make(chan os.Signal), func() {} })
	j.SetStdin(strings.NewReader("\n"))
	j.SetStdout(&out)
	j.Run()
	if !strings.Contains(out.String(), "APT: removing meld") || !strings.Contains(out.String(), "Finished: 2 removed.") {
		t.Errorf("output = %s", out.String())
	}
	// Seen on Windows 11: a removal said "if the installer needs it".
	if !strings.Contains(out.String(), "if the uninstaller needs it") || strings.Contains(out.String(), "if the installer needs it") {
		t.Errorf("removal should mention the uninstaller: %s", out.String())
	}
	// Each handover starts with a rule, so it can't be confused with the
	// previous handover's output still on screen.
	if !strings.HasPrefix(out.String(), "\n──── ") {
		t.Errorf("handover should start with a rule: %q", out.String()[:min(40, len(out.String()))])
	}
	if Summary([]core.StepResult{{Step: core.UpdateStep{Action: core.ActionClean}, Status: core.StepDone}}) != "Finished: 1 cleaned up." {
		t.Error("clean-up summary wording")
	}
}

// Seen on Windows 11: answering No to the permission prompt made winget exit
// with 0x8A15010C, shown as "Didn't work (exit code 2316632332)".
func TestDecliningPermissionIsCancelledNotFailed(t *testing.T) {
	codes := map[string]int{"x": negHRESULT, "y": 0x8A150001, "z": 0}
	start := func(c core.Command, in io.Reader, out, errw io.Writer) (int, error) {
		return codes[c.Program], nil
	}
	var out bytes.Buffer
	j := New([]core.UpdateStep{
		{Label: "Windows Package Manager", Action: core.ActionInstall, Updates: []core.Update{{Name: "Notepad++"}}, Commands: []core.Command{{Program: "x"}}},
		{Label: "Windows Package Manager", Action: core.ActionInstall, Updates: []core.Update{{Name: "Broken"}}, Commands: []core.Command{{Program: "y"}}},
		{Label: "Windows Package Manager", Action: core.ActionInstall, Updates: []core.Update{{Name: "Fine"}}, Commands: []core.Command{{Program: "z"}}},
	}, start, func() (<-chan os.Signal, func()) { return make(chan os.Signal), func() {} })
	j.SetStdin(strings.NewReader("\n"))
	j.SetStdout(&out)
	j.Run()
	r := j.Results()
	if r[0].Status != core.StepStopped || !strings.Contains(r[0].Note, "You cancelled it") {
		t.Errorf("declined: %+v", r[0])
	}
	if r[1].Status != core.StepFailed || r[2].Status != core.StepDone {
		t.Errorf("a cancelled step must not stop the others: %+v", r)
	}
	for _, w := range []string{"Cancelled: You cancelled it", "Didn't work (exit code 0x8A150001)"} {
		if !strings.Contains(out.String(), w) {
			t.Errorf("output missing %q:\n%s", w, out.String())
		}
	}
}

// negHRESULT is 0x8A15010C as Windows returns it to Go on 64-bit: a negative
// 32-bit value.
var negHRESULT = int(int32(-1978334964))
