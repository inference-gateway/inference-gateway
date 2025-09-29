FROM golang:1.25-alpine
WORKDIR /app
COPY . /app
RUN apk add --no-cache curl
RUN go mod download
RUN go build -o search-server main.go
EXPOSE 8082
CMD ["./search-server"]
