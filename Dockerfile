# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build

WORKDIR /workspace

COPY go.mod ./
RUN go mod download

COPY . ./
RUN timeout -k 10s 300s go test ./... && \
    CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags='-s -w' -o /artifact/pages .

FROM alpine:3.22

RUN addgroup -S -g 10001 pages && \
    adduser -S -D -H -u 10001 -G pages pages

COPY --from=build --chown=10001:10001 /artifact/pages /app/pages

USER 10001:10001

ENV PAGES_LISTEN_ADDR=0.0.0.0:3103 \
    PAGES_DESTINATION=/srv/pages/public \
    PAGES_TOKENS_FILE=/run/secrets/pages_tokens \
    PAGES_MAX_UPLOAD_BYTES=10485760

EXPOSE 3103

ENTRYPOINT ["/app/pages", "serve"]
