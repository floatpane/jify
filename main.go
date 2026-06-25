// Command jify shows native emoji suggestions whenever you type a shortcode
// (e.g. ":smile") in any application.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/floatpane/jify/internal/native"
	"github.com/floatpane/jify/pkg/autostart"
	"github.com/floatpane/jify/pkg/config"
	"github.com/floatpane/jify/pkg/emoji"
)

// daemonEnv marks the detached background child so it doesn't re-spawn itself.
const daemonEnv = "JIFY_DAEMON"

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
	foreground := false
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-f", "--foreground":
			foreground = true
		default:
			if handleCommand(os.Args[1]) {
				return
			}
		}
	}

	// When launched interactively from a terminal, re-spawn detached so the
	// shell prompt returns immediately. The child carries JIFY_DAEMON=1 and runs
	// in the foreground. When launched by launchd/systemd/snap (no controlling
	// terminal), run in the foreground so the supervisor can track the process.
	if !foreground && os.Getenv(daemonEnv) == "" && isInteractive() {
		if err := daemonize(); err != nil {
			log.Fatalf("jify: could not start in background: %v", err)
		}
		return
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

// isInteractive reports whether jify was started from a terminal (stdin is a
// character device).
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// daemonize re-executes jify as a detached background process, redirecting its
// output to a log file, and returns once the child has started.
func daemonize() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = append(os.Environ(), daemonEnv+"=1")
	cmd.SysProcAttr = detachSysProcAttr()
	cmd.Stdin = nil

	if logFile := openDaemonLog(); logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		defer logFile.Close()
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	fmt.Printf("jify started in the background (pid %d)\n", cmd.Process.Pid)
	return cmd.Process.Release()
}

// openDaemonLog opens (creating if needed) the background log file next to the
// config. Returns nil if it can't be opened; jify still runs without logging.
func openDaemonLog() *os.File {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	dir = filepath.Join(dir, "jify")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(dir, "jify.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil
	}
	return f
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
  jify              Start jify in the background (type ":name" anywhere).
  jify -f           Run in the foreground (don't detach; useful for debugging).
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
