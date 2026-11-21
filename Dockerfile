FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd cmd
COPY internal internal
COPY scripts/build-agent-releases.py scripts/fetch-engines.py ./scripts/
RUN apk add --no-cache python3 ca-certificates && python3 scripts/build-agent-releases.py --output /out-agent-releases
RUN CGO_ENABLED=0 go build -trimpath -o /out/control ./cmd/control && CGO_ENABLED=0 go build -trimpath -o /out/agent ./cmd/agent
FROM alpine:3.22
RUN apk add --no-cache ca-certificates wget && addgroup -S app && adduser -S app -G app && mkdir /state && chown app:app /state
COPY --from=build /out/ /app/
COPY --from=build /out-agent-releases/ /app/agent-releases/
USER app
ENTRYPOINT ["/app/control"]
