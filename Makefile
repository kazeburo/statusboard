GIT_VER := $(shell git describe --tags)
LDFLAGS=-ldflags "-w -s -X main.version=${GIT_VER}"

all: statusboard

.PHONY: statusboard

statusboard: logs.go toml.go worker.go handlers.go main.go
	go build $(LDFLAGS) -o statusboard

linux: logs.go toml.go worker.go handlers.go main.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o statusboard

check:
	go test -v
	go test -race