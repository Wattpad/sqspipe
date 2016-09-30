SOURCEDIR=.
SOURCES := $(shell find $(SOURCEDIR) -name '*.go')
VERSION := $(shell git describe --always --dirty)

.PHONY: container push version clean

sqspipe: $(SOURCES)
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version=${VERSION}" .

container: sqspipe
	docker build -t jharlap/sqspipe:$(VERSION) .
	docker tag jharlap/sqspipe:$(VERSION) jharlap/sqspipe:latest

push: container
	docker push jharlap/sqspipe:$(VERSION)
	docker push jharlap/sqspipe:latest

version:
	@echo $(VERSION)

clean:
	rm sqspipe
