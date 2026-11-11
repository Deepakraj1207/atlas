// Command atlas is the ATLAS terminal application.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/deepakraj1207/atlas/internal/app"
)

// version is set at build time: -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	configPath := flag.String("config", "", "path to the settings file (default: per-user config folder)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "ATLAS — understand and manage the software on this computer.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage: atlas [flags]")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("atlas", version)
		return
	}

	if err := app.Run(app.Options{ConfigPath: *configPath, Version: version}); err != nil {
		fmt.Fprintln(os.Stderr, "ATLAS could not start:", err)
		os.Exit(1)
	}
}
