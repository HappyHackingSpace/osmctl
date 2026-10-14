.PHONY: build test lint docs check

build:
	go build -o osmctl ./cmd/osmctl

test:
	go test -race ./...

lint:
	golangci-lint run ./...

docs:
	go run ./tools/gendocs

check: lint test docs
	git diff --exit-code docs/commands
