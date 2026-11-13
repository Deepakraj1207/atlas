// Command conharness runs a program inside a real Windows pseudo-console (ConPTY),
// feeds it keystrokes from a script and records what the screen shows.
//
// Usage: conharness -cols 110 -rows 34 -script keys.txt -out screens.txt -- prog args...
//
// Script lines:
//
//	wait <ms>
//	until [seconds] <text>   wait until the screen contains text (default 60 s)
//	gone [seconds] <text>    wait until the screen no longer contains text
//	keys <k> <k> ...         enter esc tab up down left right space bs ctrl+c, or one character
//	type <text>              literal text
//	snap <label>             record the screen
//	expect <text>            the screen must contain text now
//	absent <text>            the screen must not contain text now
//
// In until, gone, expect and absent, "a || b" means either text: for
// outcomes that depend on the computer, such as a scan with gaps.
//
//	resize <cols> <rows>
//	# comment
//
// With -strict, a timeout, a failed expect/absent or a non-zero exit code
// from the program makes conharness exit with 1.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/hinshun/vt10x"
	"golang.org/x/sys/windows"
)

var keyMap = map[string]string{
	"enter": "\r", "esc": "\x1b", "tab": "\t", "up": "\x1b[A", "down": "\x1b[B",
	"right": "\x1b[C", "left": "\x1b[D", "space": " ", "bs": "\x7f", "ctrl+c": "\x03",
	"pgdn": "\x1b[6~", "pgup": "\x1b[5~", "home": "\x1b[H", "end": "\x1b[F",
}

func main() {
	cols := flag.Int("cols", 110, "")
	rows := flag.Int("rows", 34, "")
	script := flag.String("script", "", "")
	outPath := flag.String("out", "screens.txt", "")
	rawPath := flag.String("raw", "", "")
	exitWait := flag.Int("exitwait", 10, "seconds to wait for exit after the script")
	strict := flag.Bool("strict", false, "exit 1 on any timeout, failed check or non-zero exit code")
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "no program")
		os.Exit(2)
	}
	out, err := os.Create(*outPath)
	must(err)
	defer out.Close()
	var raw *os.File
	if *rawPath != "" {
		raw, _ = os.Create(*rawPath)
	}
	failures := 0
	logf := func(format string, a ...any) {
		s := fmt.Sprintf(format, a...)
		if strings.HasPrefix(s, "!! ") {
			failures++
		}
		fmt.Fprintln(out, s)
		fmt.Println(s)
	}
	finish := func() {
		if *strict && failures > 0 {
			fmt.Printf("conharness: %d problem(s)\n", failures)
			os.Exit(1)
		}
	}

	var inR, inW, outR, outW windows.Handle
	must(windows.CreatePipe(&inR, &inW, nil, 0))
	must(windows.CreatePipe(&outR, &outW, nil, 0))
	var hpc windows.Handle
	must(windows.CreatePseudoConsole(windows.Coord{X: int16(*cols), Y: int16(*rows)}, inR, outW, 0, &hpc))
	windows.CloseHandle(inR)
	windows.CloseHandle(outW)

	attrs, err := windows.NewProcThreadAttributeList(1)
	must(err)
	must(attrs.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, unsafe.Pointer(hpc), unsafe.Sizeof(hpc)))
	si := &windows.StartupInfoEx{ProcThreadAttributeList: attrs.List()}
	si.Cb = uint32(unsafe.Sizeof(*si))
	si.Flags = windows.STARTF_USESTDHANDLES // no std handles: the child uses the pseudo-console
	cmdline := makeCmdLine(args)
	var pi windows.ProcessInformation
	must(windows.CreateProcess(nil, windows.StringToUTF16Ptr(cmdline), nil, nil, false,
		windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_UNICODE_ENVIRONMENT, nil, nil, &si.StartupInfo, &pi))
	windows.CloseHandle(pi.Thread)

	term := vt10x.New(vt10x.WithSize(*cols, *rows))
	var mu sync.Mutex
	go func() {
		buf := make([]byte, 64*1024)
		for {
			var n uint32
			if err := windows.ReadFile(outR, buf, &n, nil); err != nil || n == 0 {
				return
			}
			if raw != nil {
				raw.Write(buf[:n])
			}
			mu.Lock()
			term.Write(buf[:n])
			mu.Unlock()
		}
	}()
	screen := func() string {
		mu.Lock()
		defer mu.Unlock()
		lines := strings.Split(term.String(), "\n")
		for i := range lines {
			lines[i] = strings.TrimRight(lines[i], " ")
		}
		return strings.TrimRight(strings.Join(lines, "\n"), "\n")
	}
	send := func(s string) {
		var n uint32
		windows.WriteFile(inW, []byte(s), &n, nil)
	}
	exited := func() (bool, uint32) {
		ev, _ := windows.WaitForSingleObject(pi.Process, 0)
		if ev == windows.WAIT_OBJECT_0 {
			var code uint32
			windows.GetExitCodeProcess(pi.Process, &code)
			return true, code
		}
		return false, 0
	}
	snap := func(label string) {
		logf("===== %s (%s) =====", label, time.Now().Format("15:04:05"))
		logf("%s", screen())
		logf("===== end %s =====", label)
	}

	lines := []string{}
	if *script != "" {
		b, err := os.ReadFile(*script)
		must(err)
		lines = strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	}
	start := time.Now()
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cmd, rest, _ := strings.Cut(line, " ")
		if done, code := exited(); done && cmd != "snap" {
			logf("!! program exited with code %d before: %s", code, line)
			break
		}
		switch cmd {
		case "wait":
			ms, _ := strconv.Atoi(rest)
			time.Sleep(time.Duration(ms) * time.Millisecond)
		case "until", "gone":
			secs := 60
			if f, r, ok := strings.Cut(rest, " "); ok {
				if n, err := strconv.Atoi(f); err == nil {
					secs, rest = n, r
				}
			}
			deadline := time.Now().Add(time.Duration(secs) * time.Second)
			for {
				has := containsAny(screen(), rest)
				if (cmd == "until") == has {
					logf("-- %s %q ok after %.1fs", cmd, rest, time.Since(start).Seconds())
					break
				}
				if done, _ := exited(); done || time.Now().After(deadline) {
					logf("!! TIMEOUT %s %q", cmd, rest)
					snap("timeout")
					break
				}
				time.Sleep(200 * time.Millisecond)
			}
		case "keys":
			for _, k := range strings.Fields(rest) {
				if s, ok := keyMap[k]; ok {
					send(s)
				} else {
					send(k)
				}
				time.Sleep(250 * time.Millisecond)
			}
		case "type":
			for _, r := range rest {
				send(string(r))
				time.Sleep(60 * time.Millisecond)
			}
		case "snap":
			time.Sleep(300 * time.Millisecond)
			snap(rest)
		case "expect", "absent":
			has := containsAny(screen(), rest)
			if (cmd == "expect") != has {
				logf("!! FAILED %s %q", cmd, rest)
				snap("failed check")
			} else {
				logf("-- %s %q ok", cmd, rest)
			}
		case "resize":
			f := strings.Fields(rest)
			c, _ := strconv.Atoi(f[0])
			r, _ := strconv.Atoi(f[1])
			windows.ResizePseudoConsole(hpc, windows.Coord{X: int16(c), Y: int16(r)})
			mu.Lock()
			term.Resize(c, r)
			mu.Unlock()
			time.Sleep(500 * time.Millisecond)
		default:
			logf("!! unknown script line: %s", line)
		}
	}
	deadline := time.Now().Add(time.Duration(*exitWait) * time.Second)
	for time.Now().Before(deadline) {
		if done, code := exited(); done {
			if code != 0 {
				logf("!! program exited with code %d", code)
			} else {
				logf("-- program exited with code 0")
			}
			time.Sleep(300 * time.Millisecond)
			snap("after exit")
			windows.ClosePseudoConsole(hpc)
			finish()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	logf("!! program still running after script; stopping it")
	snap("final")
	windows.TerminateProcess(pi.Process, 1)
	windows.ClosePseudoConsole(hpc)
	finish()
}

// containsAny reports whether s contains any of the " || "-separated texts.
func containsAny(s, alternatives string) bool {
	for _, a := range strings.Split(alternatives, " || ") {
		if strings.Contains(s, a) {
			return true
		}
	}
	return false
}

func makeCmdLine(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		q[i] = syscall.EscapeArg(a)
	}
	return strings.Join(q, " ")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "conharness:", err)
		os.Exit(2)
	}
}
