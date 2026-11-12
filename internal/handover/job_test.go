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
	}, start, func() (<-chan os.Signal, func()) { return make(chan os.Signal), func() {} })
	j.SetStdin(strings.NewReader("\n"))
	j.SetStdout(&out)
	j.Run()
	if !strings.Contains(out.String(), "APT: removing meld") || !strings.Contains(out.String(), "Finished: 1 removed.") {
		t.Errorf("output = %s", out.String())
	}
	if Summary([]core.StepResult{{Step: core.UpdateStep{Action: core.ActionClean}, Status: core.StepDone}}) != "Finished: 1 cleaned up." {
		t.Error("clean-up summary wording")
	}
}
