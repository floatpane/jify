BINARY := jify
VERSION ?= dev

.PHONY: build run test vet clean windows macos-app icons windows-syso

build:
	CGO_ENABLED=1 go build -o $(BINARY) .

# Windows build with the GUI subsystem so no console window appears.
windows:
	GOOS=windows CGO_ENABLED=0 go build -ldflags="-H windowsgui" -o $(BINARY).exe .

# Bundle jify.app (macOS) with the icon into ./dist.
macos-app:
	./scripts/package-macos-app.sh $(VERSION) dist

run: build
	./$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

# Regenerate the Windows icon/version resource objects from assets/jify.ico.
windows-syso:
	go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.7.0 -64 -o resource_windows_amd64.syso versioninfo.json
	go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.7.0 -64 -arm -o resource_windows_arm64.syso versioninfo.json

# Regenerate all icon assets from assets/logo-1024.png.
icons:
	./scripts/gen-icons.sh

# Regenerate pkg/emoji/emoji.json from the gemoji database.
emoji:
	./scripts/gen-emoji.sh

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf dist
