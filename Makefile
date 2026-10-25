.PHONY: build test install clean

VERSION ?= $(shell git describe --tags --exact-match --match 'v[0-9]*' 2>/dev/null | sed 's/^v//')
LDFLAGS := -s -w $(if $(VERSION),-X github.com/btraven00/gorxiv/cmd.version=$(VERSION))

build: ## Build bin/gorxiv
	go build -o bin/gorxiv -ldflags="$(LDFLAGS)" .

test:
	go test ./...

install:
	go install -ldflags="$(LDFLAGS)" .

clean:
	rm -rf bin
