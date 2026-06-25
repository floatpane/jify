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

- **Works everywhere** — a global key event tap watches your typing system-wide.
- **Native popup** — uses the macOS "liquid glass" (`NSGlassEffectView`) on
  macOS 26+, falling back to a frosted `NSVisualEffectView` on older releases.
- **Configurable** — trigger character, result count, theme and per-app
  blacklist all live in a JSON config.
- **App blacklist** — disable jify in specific apps (e.g. password managers).

## Install / build

Requires Go 1.24+ and the Xcode command line tools (for cgo / Objective-C).

```sh
go build -o jify .
```

## Usage

```sh
./jify            # run jify (type ":name" anywhere)
./jify config     # print the config file path
./jify help       # show help
```

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

## Platform support

| Platform | Status                                                            |
| -------- | ---------------------------------------------------------------- |
| macOS    | ✅ Implemented (`internal/native/darwin.m`).                      |
| Linux    | ⏳ Stub — add a backend behind a build tag (X11/Wayland + GTK).   |
| Windows  | ⏳ Stub — add a backend behind a build tag (Win32 hook + WinUI).  |

The Go core (`pkg/config`, `pkg/emoji`) is platform-independent; each OS provides
a keyboard hook + popup that call the exported `jifyQuery` / `jifyIsBlacklisted`
callbacks.
