// Package handover runs steps in the user's own terminal. ATLAS hands the
// terminal to each package manager in turn, so its real output and prompts
// (passwords, permission dialogs, confirmations) reach the user directly.
package handover

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync/atomic"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

// ProcessStarter runs one command attached to the given streams and
// returns its exit code. err is set only when the command couldn't run.
type ProcessStarter func(c core.Command, stdin io.Reader, stdout, stderr io.Writer) (int, error)

// ExecStarter runs a real process. When the streams are the terminal's
// files the child gets the real terminal, so its prompts work normally.
func ExecStarter(c core.Command, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	path, err := exec.LookPath(c.Program)
	if err != nil {
		return -1, fmt.Errorf("%s isn't installed or isn't on PATH", c.Program)
	}
	cmd := exec.Command(path, c.Args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	err = cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0, nil
	case errors.As(err, &exitErr):
		return exitErr.ExitCode(), nil
	default:
		return -1, err
	}
}

// NotifyInterrupt delivers Ctrl+C to the job while it runs.
func NotifyInterrupt() (<-chan os.Signal, func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	return ch, func() { signal.Stop(ch) }
}

// New prepares a job. start and notify may be nil for the defaults.
func New(steps []core.UpdateStep, start ProcessStarter, notify func() (<-chan os.Signal, func())) *Job {
	if start == nil {
		start = ExecStarter
	}
	if notify == nil {
		notify = NotifyInterrupt
	}
	return &Job{steps: steps, start: start, notify: notify}
}

// Job runs steps in the user's terminal. It implements core.TerminalJob
// (and so Bubble Tea's ExecCommand).
type Job struct {
	steps  []core.UpdateStep
	start  ProcessStarter
	notify func() (<-chan os.Signal, func())

	stdin          io.Reader
	stdout, stderr io.Writer
	results        []core.StepResult
}

func (j *Job) SetStdin(r io.Reader)  { j.stdin = r }
func (j *Job) SetStdout(w io.Writer) { j.stdout = w }
func (j *Job) SetStderr(w io.Writer) { j.stderr = w }

// Results returns one result per step, in order. Valid after Run.
func (j *Job) Results() []core.StepResult { return j.results }

func (j *Job) printf(format string, args ...any) {
	if j.stdout != nil {
		fmt.Fprintf(j.stdout, format, args...)
	}
}

// Run executes each step's commands in order. Ctrl+C reaches the running
// package manager as usual; ATLAS catches it too, lets the current command
// finish stopping, and starts nothing further.
func (j *Job) Run() error {
	if j.stdin == nil {
		j.stdin = os.Stdin
	}
	if j.stdout == nil {
		j.stdout = os.Stdout
	}
	if j.stderr == nil {
		j.stderr = os.Stderr
	}
	var stopped atomic.Bool
	if j.notify != nil {
		sig, stop := j.notify()
		defer stop()
		done := make(chan struct{})
		defer close(done)
		go func() {
			select {
			case <-sig:
				stopped.Store(true)
			case <-done:
			}
		}()
	}

	j.printf("\nATLAS is handing this terminal to your package managers.\n")
	j.printf("You'll see their own messages below. Answer any questions they ask; ATLAS never answers for you.\n")
	j.printf("Press Ctrl+C to stop. Nothing further will start after that.\n")

	runnable := 0
	for _, st := range j.steps {
		if st.Blocked == "" {
			runnable++
		}
	}
	n := 0
	j.results = make([]core.StepResult, 0, len(j.steps))
	for _, st := range j.steps {
		res := core.StepResult{Step: st}
		switch {
		case st.Blocked != "":
			res.Status, res.Note = core.StepSkipped, st.Blocked
			j.printf("\nSkipped %s: %s\n", describe(st), st.Blocked)
			j.results = append(j.results, res)
			continue
		case stopped.Load():
			res.Status, res.Note = core.StepStopped, "Not started because you stopped the updates"
			j.results = append(j.results, res)
			continue
		}
		n++
		j.printf("\n[%d/%d] %s: %s %s\n", n, runnable, st.Label, verb(st.Action), describe(st))
		switch st.Elevation {
		case core.ElevationSudo:
			j.printf("      sudo may ask for your password. ATLAS never sees or stores it.\n")
		case core.ElevationInstaller:
			j.printf("      Windows may ask for permission if the installer needs it.\n")
		case core.ElevationPolkit:
			j.printf("      Your system may ask for your password.\n")
		}
		begin := time.Now()
		res.Status = core.StepDone
		for _, c := range st.Commands {
			res.Command = c.String()
			j.printf("$ %s\n", res.Command)
			code, err := j.start(c, j.stdin, j.stdout, j.stderr)
			res.ExitCode = code
			if err != nil {
				res.Status, res.Note = core.StepFailed, err.Error()
				break
			}
			ok, restart := c.Interpret(code)
			res.Restart = res.Restart || restart
			if !ok {
				res.Status = core.StepFailed
				if stopped.Load() {
					res.Status, res.Note = core.StepStopped, "Stopped by you"
				}
				break
			}
		}
		res.Duration = time.Since(begin)
		if res.Restart {
			res.Note = "Restart needed to finish"
		}
		j.printf("%s\n", resultLine(res))
		j.results = append(j.results, res)
	}

	j.printf("\n%s\n", Summary(j.results))
	j.printf("Press Enter to return to ATLAS.")
	_, _ = bufio.NewReader(j.stdin).ReadString('\n')
	return nil
}

func describe(st core.UpdateStep) string {
	if len(st.Updates) == 1 {
		u := st.Updates[0]
		switch {
		case u.CurrentVersion != "" && u.AvailableVersion != "":
			return fmt.Sprintf("%s (%s → %s)", u.Name, u.CurrentVersion, u.AvailableVersion)
		case u.AvailableVersion != "":
			return fmt.Sprintf("%s (→ %s)", u.Name, u.AvailableVersion)
		default:
			return u.Name
		}
	}
	return fmt.Sprintf("%d packages", len(st.Updates))
}

func resultLine(r core.StepResult) string {
	switch r.Status {
	case core.StepDone:
		if r.Restart {
			return "  Done. A restart is needed to finish."
		}
		return "  Done."
	case core.StepStopped:
		return "  Stopped."
	default:
		if r.Note != "" {
			return "  Didn't work: " + r.Note
		}
		return fmt.Sprintf("  Didn't work (exit code %d). The message above explains why.", r.ExitCode)
	}
}

func verb(action string) string {
	switch action {
	case core.ActionRemove:
		return "removing"
	case core.ActionClean:
		return "cleaning up"
	case core.ActionInstall:
		return "installing"
	default:
		return "updating"
	}
}

func doneWord(results []core.StepResult) string {
	action := ""
	for i, r := range results {
		if i == 0 {
			action = r.Step.Action
		} else if r.Step.Action != action {
			return "done"
		}
	}
	switch action {
	case core.ActionRemove:
		return "removed"
	case core.ActionClean:
		return "cleaned up"
	case core.ActionInstall:
		return "installed"
	default:
		return "updated"
	}
}

// Summary is the one-line result printed at the end of a run.
func Summary(results []core.StepResult) string {
	count := map[core.StepStatus]int{}
	for _, r := range results {
		count[r.Status] += max(len(r.Step.Updates), 1)
	}
	var parts []string
	for _, p := range []struct {
		s     core.StepStatus
		label string
	}{{core.StepDone, doneWord(results)}, {core.StepFailed, "didn't work"}, {core.StepStopped, "stopped"}, {core.StepSkipped, "skipped"}} {
		if count[p.s] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count[p.s], p.label))
		}
	}
	if len(parts) == 0 {
		return "Nothing was run."
	}
	return "Finished: " + strings.Join(parts, ", ") + "."
}
