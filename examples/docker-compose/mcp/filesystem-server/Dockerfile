FROM golang:1.25-alpine
WORKDIR /app
COPY . /app
RUN apk add --no-cache curl
RUN go mod download
RUN go build -o filesystem-server main.go
EXPOSE 8083
CMD ["./filesystem-server"]
