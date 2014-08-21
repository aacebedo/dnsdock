dnsdock: *.go | deps lint
	go build

deps:
	go get

test: | lint
	go test -v

lint:
	go fmt

install:
	go install -ldflags "-X main.version `git describe --tags HEAD``if  git status --porcelain --untracked-files=no >/dev/null ; then echo "-dirty"; fi`" ./...

.PHONY: deps test lint install