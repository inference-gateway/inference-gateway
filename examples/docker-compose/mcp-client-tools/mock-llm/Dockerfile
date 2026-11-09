FROM golang:1.27.3-alpine
WORKDIR /app
COPY . /app
RUN go build -o mock-llm main.go
EXPOSE 8080
CMD ["./mock-llm"]
