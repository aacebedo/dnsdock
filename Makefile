dnsdock: *.go | deps lint
	go build

deps:
	go get

test: | lint
	go test -v

lint:
	go fmt

.PHONY: deps test lint