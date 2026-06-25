// Command jify shows native emoji suggestions whenever you type a shortcode
// (e.g. ":smile") in any application.
package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/floatpane/jify/internal/native"
	"github.com/floatpane/jify/pkg/autostart"
	"github.com/floatpane/jify/pkg/config"
	"github.com/floatpane/jify/pkg/emoji"
)

// Build information, injected at release time via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {
	// The macOS event tap and AppKit run loop must own the main OS thread.
	runtime.LockOSThread()
}

func main() {
	if len(os.Args) > 1 {
		if handleCommand(os.Args[1]) {
			return
		}
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("jify: failed to load config: %v", err)
	}

	// Reconcile login-item state with the config on every launch so the OS
	// matches the user's preference (autostart defaults to true).
	if err := autostart.Sync(cfg.Autostart); err != nil {
		log.Printf("jify: could not update autostart: %v", err)
	}

	db := emoji.NewDatabase()

	if err := native.Run(cfg, db); err != nil {
		log.Fatalf("jify: %v", err)
	}
}

// handleCommand runs a CLI subcommand and reports whether it handled os.Args.
func handleCommand(cmd string) bool {
	switch cmd {
	case "config", "config-path":
		path, err := config.Path()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(path)
	case "enable", "enable-autostart":
		setAutostart(true)
	case "disable", "disable-autostart":
		setAutostart(false)
	case "status":
		printStatus()
	case "version", "--version", "-v":
		fmt.Printf("jify %s (commit %s, built %s)\n", version, commit, date)
	case "-h", "--help", "help":
		usage()
	default:
		return false
	}
	return true
}

// setAutostart persists the preference and applies it immediately.
func setAutostart(enabled bool) {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("jify: %v", err)
	}
	cfg.Autostart = enabled
	if err := cfg.Save(); err != nil {
		log.Fatalf("jify: %v", err)
	}
	if err := autostart.Sync(enabled); err != nil {
		log.Fatalf("jify: %v", err)
	}
	if enabled {
		fmt.Println("jify will now start automatically at login.")
	} else {
		fmt.Println("jify will no longer start automatically at login.")
	}
}

func printStatus() {
	on, err := autostart.IsEnabled()
	if err != nil {
		log.Fatalf("jify: %v", err)
	}
	if on {
		fmt.Println("autostart: enabled")
	} else {
		fmt.Println("autostart: disabled")
	}
}

func usage() {
	fmt.Print(`jify - native shortcode emoji picker

Usage:
  jify              Run jify in the background (type ":name" anywhere).
  jify enable       Start jify automatically at login.
  jify disable      Stop starting jify at login.
  jify status       Show whether autostart is enabled.
  jify config       Print the path to the config file.
  jify version      Print version information.
  jify help         Show this help.

Config file fields (JSON):
  trigger           Character that starts a suggestion session (default ":").
  maxSuggestions    Maximum emojis shown in the popup.
  blacklistedApps   App bundle ids / names where jify stays disabled.
  theme             "native", "dark" or "light".
  autostart         Launch jify at login (default true).
`)
}
