# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src

COPY cli/go.mod cli/go.sum ./cli/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd cli && go mod download

COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -o /out/tasker-mcp-server ./cli

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app

COPY --chown=nonroot:nonroot --from=builder /out/tasker-mcp-server /app/tasker-mcp-server
COPY --chown=nonroot:nonroot dist/toolDescriptions.json /app/dist/toolDescriptions.json

EXPOSE 8000
ENTRYPOINT ["/app/tasker-mcp-server"]
CMD ["--mode","sse","--host","0.0.0.0","--port","8000","--tools","/app/dist/toolDescriptions.json"]
