FROM golang:1.26.4-alpine3.23 AS builder
WORKDIR /build
RUN apk add --no-cache git
COPY go.mod go.sum ./
COPY vendor .
COPY .. .
RUN mkdir bin
# GOEXPERIMENT=jsonv2 required by github.com/lestrrat-go/jwx/v4
RUN GOEXPERIMENT=jsonv2 go build -mod vendor -o bin ./cmd/controlplane

FROM alpine:3.23
RUN apk add --no-cache ca-certificates musl-locales musl-locales-lang
# we deal with files and we just want to make sure that no strange shit
# happens because LANG does not support utf8
ENV LANG=en_US.UTF-8
ENV LC_ALL=en_US.UTF-8
WORKDIR /bin
COPY --from=builder /build/bin/controlplane controlplane
ENTRYPOINT ["/bin/controlplane"]
