# syntax=docker/dockerfile:1
FROM golang:1.27-alpine AS agent-builder
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/nodehub-agent ./cmd/nodehub-agent
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /nodehub-agent ./cmd/nodehub-agent
RUN go test ./cmd/nodehub-agent

FROM alpine:3.22
ARG TARGETARCH
ARG XRAY_VERSION=26.10.31
RUN apk add --no-cache ca-certificates tzdata curl unzip \
    && case "$TARGETARCH" in amd64) archive=64;; arm64) archive=arm64-v8a;; arm) archive=arm32-v7a;; 386) archive=32;; *) echo "unsupported architecture: $TARGETARCH" >&2; exit 1;; esac \
    && curl -fsSL "https://github.com/XTLS/Xray-core/releases/download/v${XRAY_VERSION}/Xray-linux-${archive}.zip" -o /tmp/xray.zip \
    && unzip -q /tmp/xray.zip xray -d /usr/local/bin \
    && rm /tmp/xray.zip \
    && addgroup -S -g 10001 nodehub \
    && adduser -S -D -H -u 10001 -s /sbin/nologin -G nodehub nodehub \
    && chmod 0755 /usr/local/bin/xray
COPY --from=agent-builder /nodehub-agent /usr/local/bin/nodehub-agent
RUN chmod 0755 /usr/local/bin/nodehub-agent
USER nodehub
WORKDIR /var/lib/nodehub-agent
VOLUME ["/var/lib/nodehub-agent"]
EXPOSE 9443
ENTRYPOINT ["/usr/local/bin/nodehub-agent"]
