FROM golang:1.23-alpine
WORKDIR /app
COPY . /app
RUN apk add --no-cache curl
RUN go mod download
RUN go build -o time-server main.go
EXPOSE 8081
CMD ["./time-server"]