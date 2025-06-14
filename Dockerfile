FROM golang:1.24.3-alpine3.21 AS builder
WORKDIR /build
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY .. .
RUN mkdir bin
RUN go build -o bin ./cmd/conncheck ./cmd/controlplane

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /prog
COPY --from=builder /build/bin/conncheck conncheck
COPY --from=builder /build/bin/controlplane controlplane
ENTRYPOINT ["/prog/controlplane"]
