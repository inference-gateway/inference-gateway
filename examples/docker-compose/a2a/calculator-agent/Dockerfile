FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache curl
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build  -ldflags="-w -s" -o calculator-agent .

FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache curl
COPY --from=builder /app/calculator-agent .
EXPOSE 8082
CMD ["./calculator-agent"]
