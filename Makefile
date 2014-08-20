dnsdock: *.go | deps
	go build

deps:
	go get

test:
	go test -v

lint:
	go fmt

.PHONY: deps test