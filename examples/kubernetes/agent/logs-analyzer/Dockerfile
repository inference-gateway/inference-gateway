FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o log-analyser main.go

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /app/log-analyser .
EXPOSE 8080
CMD ["./log-analyser"]
