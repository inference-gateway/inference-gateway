FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache curl
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o weather-agent .

FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache curl
COPY --from=builder /app/weather-agent .
EXPOSE 8083
CMD ["./weather-agent"]
