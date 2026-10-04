FROM golang:1.26.8-alpine
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o calculator-server .
EXPOSE 8085
CMD ["./calculator-server"]
