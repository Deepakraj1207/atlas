// Command atlas is the ATLAS terminal application.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/deepakraj1207/atlas/internal/app"
	"github.com/deepakraj1207/atlas/internal/selftest"
)

// Set at build time by release builds:
//
//	-ldflags "-X main.version=0.1.0 -X main.commit=1a2b3c4 -X main.date=2026-11-13"
//
// Other builds fall back to the Git details Go records (see versionLine).
var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	configPath := flag.String("config", "", "path to the settings file (default: per-user config folder)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	runSelftest := flag.Bool("selftest", false, "run read-only checks against this computer and print a report (changes nothing)")
	reportPath := flag.String("report", "", "with -selftest: write the report to this file instead of the screen")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "ATLAS — understand and manage the software on this computer.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage: atlas [flags]")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println(versionLine(version, commit, date, buildSettings()))
		return
	}

	if *runSelftest {
		var out io.Writer = os.Stdout
		if *reportPath != "" {
			f, err := os.Create(*reportPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Couldn't create the report file:", err)
				os.Exit(1)
			}
			defer f.Close()
			out = f
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		res, err := selftest.Run(ctx, out, version)
		if err != nil {
			fmt.Fprintln(os.Stderr, "The self-test couldn't finish:", err)
			os.Exit(1)
		}
		if *reportPath != "" {
			fmt.Printf("Self-test finished: %d passed, %d warnings, %d failed. Report: %s\n", res.Passed, res.Warnings, res.Failed, *reportPath)
		}
		return
	}

	if err := app.Run(app.Options{ConfigPath: *configPath, Version: version}); err != nil {
		fmt.Fprintln(os.Stderr, "ATLAS could not start:", err)
		os.Exit(1)
	}
}
