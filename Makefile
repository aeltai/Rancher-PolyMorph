.PHONY: build test test-cover test-race lint install clean man man-view docs docs-serve

BINARY := rancher-migrate
OUT_DIR := ./bin
MAN_SRC := cmd/manual/rancher-migrate.1
VERSION := $(shell grep 'Version = ' internal/version/version.go | cut -d'"' -f2)

build:
	go build -ldflags "-s -w -X github.com/aeltai/rancher-migrate/internal/version.Version=$(VERSION)" \
		-o $(OUT_DIR)/$(BINARY) .

install: build

test:
	go test ./... -count=1

test-race:
	go test ./... -race -count=1

test-cover:
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)"

man:
	gzip -c $(MAN_SRC) > $(MAN_SRC).gz

man-view:
	man -l $(MAN_SRC)

docs:
	pip install -r docs/requirements.txt
	mkdocs build --strict

docs-serve:
	pip install -r docs/requirements.txt
	mkdocs serve

clean:
	rm -f $(OUT_DIR)/$(BINARY) $(MAN_SRC).gz coverage.out
