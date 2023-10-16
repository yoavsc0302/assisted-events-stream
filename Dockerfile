FROM registry.access.redhat.com/ubi9/go-toolset:1.18 AS build

USER root

ADD . /app

RUN cd /app && CGO_ENABLED=0 go build -ldflags='-extldflags=-static' -o=projection ./cmd/consumer/main.go

RUN cd /app && CGO_ENABLED=0 go build -ldflags='-extldflags=-static' -o=onprem ./cmd/onprem/main.go

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.2

COPY --from=build /app/projection /
COPY --from=build /app/onprem /

ENTRYPOINT ["/projection"]
