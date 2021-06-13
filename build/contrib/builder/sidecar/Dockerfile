FROM golang:1.16.5-buster AS builder

COPY . /go/src/mosn.io/mosn
WORKDIR /go/src/mosn.io/mosn

RUN echo $(git rev-parse HEAD 2> /dev/null || true)$(if ! git diff-index --quiet HEAD; then echo -dirty; fi) \
> /usr/local/bin/version.txt && CGO_ENABLED=0 go build -v -mod=vendor -o /usr/local/bin/mosn \
-ldflags="-X main.Version=$(cat /usr/local/bin/version.txt) -w -s" \
mosn.io/mosn/cmd/mosn/main 2>&1

FROM istio/proxyv2:1.10.0

COPY --from=builder /usr/local/bin/mosn /usr/local/bin/mosn
COPY --from=builder /usr/local/bin/version.txt /usr/local/bin/version.txt
