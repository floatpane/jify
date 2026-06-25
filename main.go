// Command jify shows native emoji suggestions whenever you type a shortcode
// (e.g. ":smile") in any application.
package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/floatpane/jify/internal/native"
	"github.com/floatpane/jify/pkg/config"
	"github.com/floatpane/jify/pkg/emoji"
)

func init() {
	// The macOS event tap and AppKit run loop must own the main OS thread.
	runtime.LockOSThread()
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config", "config-path":
			path, err := config.Path()
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(path)
			return
		case "-h", "--help", "help":
			usage()
			return
		}
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("jify: failed to load config: %v", err)
	}

	db := emoji.NewDatabase()

	if err := native.Run(cfg, db); err != nil {
		log.Fatalf("jify: %v", err)
	}
}

func usage() {
	fmt.Print(`jify - native shortcode emoji picker

Usage:
  jify              Run jify in the background (type ":name" anywhere).
  jify config       Print the path to the config file.
  jify help         Show this help.

Config file fields (JSON):
  trigger           Character that starts a suggestion session (default ":").
  maxSuggestions    Maximum emojis shown in the popup.
  blacklistedApps   App bundle ids / names where jify stays disabled.
  theme             "native", "dark" or "light".
`)
}
