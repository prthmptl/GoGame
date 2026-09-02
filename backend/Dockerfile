# Multi-stage build; the runtime image carries no toolchain.
FROM golang:1.24-alpine AS build

WORKDIR /src
# Copy manifests first so dependency download caches across source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/worker ./cmd/worker

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 app
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/worker /usr/local/bin/worker

USER app
EXPOSE 8080
# Overridden to /usr/local/bin/worker for the worker process group.
ENTRYPOINT ["/usr/local/bin/api"]
