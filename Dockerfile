FROM golang:1.23.4-alpine3.21 AS builder
WORKDIR /prog
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY .. .
RUN go build -o conncheck ./cmd/conncheck

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /prog
COPY --from=builder /prog/conncheck conncheck
ENTRYPOINT ["/prog/conncheck"]
