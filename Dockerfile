FROM golang:1.4.1

RUN go get -v github.com/tools/godep

ADD ./Godeps /go/src/github.com/tonistiigi/dnsdock/Godeps
WORKDIR /go/src/github.com/tonistiigi/dnsdock/
RUN godep restore

ADD . /go/src/github.com/tonistiigi/dnsdock/
RUN go install -ldflags "-X main.version `git describe --tags HEAD``if [[ -n $(command git status --porcelain --untracked-files=no 2>/dev/null) ]]; then echo "-dirty"; fi`" ./...

ENTRYPOINT ["/go/bin/dnsdock"]
