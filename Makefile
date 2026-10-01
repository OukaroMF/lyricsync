PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
VERSION ?= dev

.PHONY: build test install clean

build:
	CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o lyricsync ./cmd/lyricsync

test:
	CGO_ENABLED=0 go test -buildvcs=false ./...

install: build
	install -Dm755 lyricsync "$(DESTDIR)$(BINDIR)/lyricsync"

clean:
	rm -f lyricsync
