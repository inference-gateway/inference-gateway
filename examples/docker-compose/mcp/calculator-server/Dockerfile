FROM golang:1.27.3-alpine
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o calculator-server .
EXPOSE 8085
CMD ["./calculator-server"]
