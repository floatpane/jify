# jify

Native shortcode emoji picker. Type a trigger character (`:` by default)
followed by an emoji name in **any** application and jify pops up native
suggestions — press <kbd>Enter</kbd> to replace the shortcode with the emoji.

```
:fire        ->  🔥
:thumbsup    ->  👍
:rocket      ->  🚀
```

## Features

- **Works everywhere** — a global keyboard hook watches your typing system-wide.
- **Native popup on every OS**
  - **macOS** — "liquid glass" (`NSGlassEffectView`) on macOS 26+, falling back
    to a frosted `NSVisualEffectView` on older releases.
  - **Windows** — an acrylic-blur, rounded-corner layered window (Win10/11).
  - **Linux** — a GTK 3 popup styled with CSS (rounded, translucent, color
    emoji via Pango).
- **Configurable** — trigger character, result count, theme and per-app
  blacklist all live in a JSON config.
- **App blacklist** — disable jify in specific apps (e.g. password managers).

## Selection UX

- **macOS / Windows** — keystrokes can be intercepted, so the popup is fully
  interactive: <kbd>↑</kbd>/<kbd>↓</kbd> to move, <kbd>Enter</kbd>/<kbd>Tab</kbd>
  to insert, <kbd>Esc</kbd> to cancel.
- **Linux (X11)** — X11's `XRecord` can observe but not swallow keys, so
  selection uses the **closing-trigger** style: type `:smile:` (trigger, name,
  trigger) to insert the top suggestion. The popup updates live as you type.

## Install

**macOS (Homebrew):**

```sh
brew install floatpane/jify/jify
```

**Linux (Snap):**

```sh
sudo snap install jify --classic
```

**Windows / manual:** download the archive for your OS from the
[releases page](https://github.com/floatpane/jify/releases) and put `jify` on
your `PATH`.

## Build from source

Requires Go 1.24+.

```sh
# macOS  (needs the Xcode command line tools for cgo / Objective-C)
CGO_ENABLED=1 go build -o jify .

# Windows (pure Go, no cgo)
GOOS=windows CGO_ENABLED=0 go build -o jify.exe .

# Linux (needs gtk+-3.0, libx11 and libxtst dev packages, e.g. on Debian:
#   sudo apt install libgtk-3-dev libx11-dev libxtst-dev)
CGO_ENABLED=1 go build -o jify .
```

## Usage

```sh
./jify            # run jify in the background (type ":name" anywhere)
./jify enable     # start jify automatically at login
./jify disable    # stop starting jify at login
./jify status     # show whether autostart is enabled
./jify config     # print the config file path
./jify help       # show help
```

### Running in the background & at login

jify is designed to run as a background agent (no dock icon on macOS, no window
until the popup appears). It registers itself to launch at login automatically
on first run — controlled by the `autostart` config field, which is reconciled
with the OS every time jify starts. Use `jify disable` to turn it off.

Login mechanism per OS:

- **macOS** — a LaunchAgent at `~/Library/LaunchAgents/com.floatpane.jify.plist`.
- **Windows** — an `HKCU\…\CurrentVersion\Run` registry value.
- **Linux** — `~/.config/autostart/jify.desktop`.

> **Windows:** build with the GUI subsystem so no console window appears:
> `GOOS=windows CGO_ENABLED=0 go build -ldflags="-H windowsgui" -o jify.exe .`

On first launch macOS will ask for **Accessibility** permission
(System Settings → Privacy & Security → Accessibility). This is required for the
global key event tap. Grant it and relaunch.

- Type the trigger character followed by a name to open the popup.
- <kbd>↑</kbd>/<kbd>↓</kbd> to move the selection.
- <kbd>Enter</kbd> / <kbd>Tab</kbd> to insert the highlighted emoji.
- <kbd>Esc</kbd> or any space/punctuation cancels.

## Configuration

The config file is created with defaults on first run at:

```
~/Library/Application Support/jify/config.json   (macOS)
~/.config/jify/config.json                        (Linux)
```

See [`config.example.json`](config.example.json):

| Field             | Description                                                        |
| ----------------- | ------------------------------------------------------------------ |
| `trigger`         | Character that starts a suggestion session (default `:`).          |
| `maxSuggestions`  | Maximum emojis shown in the popup.                                 |
| `blacklistedApps` | App bundle ids (`com.apple.Terminal`) or names where jify is off.  |
| `theme`           | `native`, `dark` or `light`.                                       |
| `autostart`       | Launch jify at login (default `true`).                             |

## Platform support

| Platform | Backend                              | Hook / inject              | Popup            |
| -------- | ------------------------------------ | -------------------------- | ---------------- |
| macOS    | `darwin.m` (cgo / Objective-C)       | `CGEventTap` / `CGEvent`   | NSGlassEffectView |
| Windows  | `windows.go` (pure Go, Win32)        | `WH_KEYBOARD_LL` / SendInput | acrylic layered window |
| Linux    | `linux.c` (cgo / GTK3 + X11)         | `XRecord` / `XTest`        | GTK CSS popup    |

The Go core (`pkg/config`, `pkg/emoji`) is platform-independent. The cgo
backends (macOS, Linux) call the shared exported callbacks in `bridge.go`
(`jifyQuery`, `jifyIsBlacklisted`, `jifyTriggerRune`); the Windows backend is
pure Go and calls the core directly.

### Notes & permissions

- **macOS** needs **Accessibility** permission for the event tap.
- **Linux** requires an **X11** session (works under XWayland for most apps) with
  the `RECORD` and `XTEST` extensions, which are standard.
- **Windows** needs no special permission; if jify is used in an elevated
  (admin) window, run jify elevated too so the hook can see those keystrokes.

## Development

```sh
make test                 # unit tests
go test -tags integration ./...   # integration tests (CLI + Linux daemon smoke)
make build                # local binary
goreleaser build --snapshot --clean   # build the full release matrix locally
```

CI (`.github/workflows/ci.yml`) runs tests/build on Linux, macOS and Windows,
plus `gofmt`, `go vet`, `golangci-lint`, and config validation.
`integration.yml` runs the integration tests (Linux uses `xvfb`).

### Releasing

The **Release** workflow is manual (`workflow_dispatch`). It tags a new version,
then:

- **macOS runner** runs GoReleaser → builds darwin (clang/cgo) + windows
  (pure Go) artifacts, creates the GitHub Release, and updates the
  [Homebrew cask](https://github.com/floatpane/homebrew-jify).
- **Ubuntu runners** build native Linux tarballs (amd64/arm64) and the
  classic-confinement **snap**, publishing it to the Snap Store.

Required repository secrets:

| Secret                        | Used for                                  |
| ----------------------------- | ----------------------------------------- |
| `HOMEBREW_TAP_GITHUB_TOKEN`   | Pushing the cask to `floatpane/homebrew-jify` |
| `SNAPCRAFT_STORE_CREDENTIALS` | Publishing the snap (`snapcraft export-login`) |

> The snap name `jify` must be registered once with `snapcraft register jify`,
> and because it uses **classic** confinement the first upload needs manual
> Snap Store review approval.
