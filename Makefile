BINARY := jify

.PHONY: build run test vet clean windows

build:
	CGO_ENABLED=1 go build -o $(BINARY) .

# Windows build with the GUI subsystem so no console window appears.
windows:
	GOOS=windows CGO_ENABLED=0 go build -ldflags="-H windowsgui" -o $(BINARY).exe .

run: build
	./$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY) $(BINARY).exe
