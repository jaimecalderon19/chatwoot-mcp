FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /chatwoot-mcp ./cmd/chatwoot-mcp

FROM gcr.io/distroless/static:nonroot
COPY --from=build /chatwoot-mcp /chatwoot-mcp
USER nonroot:nonroot
ENTRYPOINT ["/chatwoot-mcp"]
