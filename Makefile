.PHONY: build test install clean man man-view

BINARY := rancher-migrate
OUT_DIR := ./bin
MAN_SRC := cmd/manual/rancher-migrate.1

build:
	go build -o $(OUT_DIR)/$(BINARY) .

install: build

test:
	go test ./...

man:
	gzip -c $(MAN_SRC) > $(MAN_SRC).gz

man-view:
	man -l $(MAN_SRC)

clean:
	rm -f $(OUT_DIR)/$(BINARY) $(MAN_SRC).gz
