FROM golang:1.26.8-alpine
WORKDIR /app
COPY . /app
RUN go build -o mock-llm main.go
EXPOSE 8080
CMD ["./mock-llm"]
