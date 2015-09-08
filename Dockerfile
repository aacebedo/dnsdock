FROM golang:1.4.1

ADD . /go/src/github.com/tonistiigi/dnsdock

RUN cd /go/src/github.com/tonistiigi/dnsdock && \
    go get -v github.com/tools/godep && \
    godep restore && \
    go install -ldflags "-X main.version `git describe --tags HEAD``git status --porcelain --untracked-files=no 2>/dev/null | grep -q ^ && echo "-dirty"`" ./...

ENTRYPOINT ["/go/bin/dnsdock"] 