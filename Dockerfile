FROM alpine:3.3

ARG GITHUB_USER="tonistigii"
ARG GOPATH="/tmp/go"
ARG WORKDIR="$GOPATH/src/github.com/$GITHUB_USER/dnsdock"

COPY . "$WORKDIR"

WORKDIR "$WORKDIR"

RUN apk --no-cache add --virtual build-deps go make git && \
    go get github.com/tools/godep && \
    "$GOPATH/bin/godep" go build -o /usr/bin/dnsdock && \
    apk del build-deps && \
    rm -rf "$GOPATH"

ENTRYPOINT ["/usr/bin/dnsdock"]
