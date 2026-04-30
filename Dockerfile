FROM golang:1.26.2-alpine3.23 AS builder
WORKDIR /build
RUN apk add --no-cache git
COPY go.mod go.sum ./
COPY vendor .
COPY .. .
RUN mkdir bin
RUN go build -mod vendor -o bin ./cmd/controlplane

FROM alpine:3.23
RUN apk add --no-cache ca-certificates
WORKDIR /bin
COPY --from=builder /build/bin/controlplane controlplane
ENTRYPOINT ["/bin/controlplane"]
