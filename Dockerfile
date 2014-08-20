FROM crosbymichael/golang

ADD . /go/src/github.com/tonistiigi/dnsdock

RUN cd /go/src/github.com/tonistiigi/dnsdock && go get -d ./... && go install ./...

ENTRYPOINT ["/go/bin/dnsdock"]