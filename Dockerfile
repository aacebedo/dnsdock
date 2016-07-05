FROM golang:1.6.2

ADD . /go/src/github.com/dnsdock
WORKDIR /go/src/github.com/dnsdock
RUN go get -v github.com/tools/godep 
RUN godep restore 
RUN bash -c 'go install -ldflags "-X main.version `git describe --tags HEAD``if [[ -n $(command git status --porcelain --untracked-files=no 2>/dev/null) ]]; then echo "-dirty"; fi`"' 

ENTRYPOINT ["/go/bin/dnsdock"] 
