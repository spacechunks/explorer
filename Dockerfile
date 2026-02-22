FROM golang:1.26.0-alpine3.23 AS builder
WORKDIR /build
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY .. .
RUN mkdir bin
RUN go build -o bin ./cmd/conncheck ./cmd/controlplane

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /bin
COPY --from=builder /build/bin/conncheck conncheck
COPY --from=builder /build/bin/controlplane controlplane
ENTRYPOINT ["/bin/controlplane"]
