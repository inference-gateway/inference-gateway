FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY config ./config
COPY logger ./logger
COPY otel ./otel
COPY cmd ./cmd
COPY api ./api
COPY providers ./providers
COPY mcp ./mcp
COPY agent ./agent
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o inference-gateway ./cmd/gateway/main.go

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /app/inference-gateway .
EXPOSE 8080
CMD ["./inference-gateway"]
