SOURCEDIR=.
SOURCES := $(shell find $(SOURCEDIR) -name '*.go')
VERSION := $(shell git describe --always --dirty)

.PHONY: container push version clean

image: $(SOURCES)
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version=${VERSION}" .

version:
	@echo $(VERSION)

clean:
	rm sqspipe
