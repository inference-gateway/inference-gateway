FROM golang:1.27.3-alpine
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o external-guardrail .
EXPOSE 8088
CMD ["./external-guardrail"]
