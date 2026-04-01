GOFLAGS := -mod=mod

.PHONY: build test clean

build:
	go build $(GOFLAGS) -o claude-code .

test:
	go test $(GOFLAGS) ./...

clean:
	rm -f claude-code

run:
	go run $(GOFLAGS) . $(ARGS)

.DEFAULT_GOAL := build
