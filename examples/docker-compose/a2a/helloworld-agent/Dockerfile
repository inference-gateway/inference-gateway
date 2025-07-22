FROM golang:1.24-alpine AS builder
ARG VERSION='0.1.0'
ARG AGENT_NAME='Hello World Agent'
ARG AGENT_DESCRIPTION='An agent that generates personalized greetings.'
WORKDIR /app
RUN apk add --no-cache curl
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X 'main.Version=${VERSION}' -X 'main.AgentName=${AGENT_NAME}' -X 'main.AgentDescription=${AGENT_DESCRIPTION}'" -o helloworld-agent .

FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache curl
COPY --from=builder /app/helloworld-agent .
COPY card.json /app/.well-known/agent.json
EXPOSE 8080
CMD ["./helloworld-agent"]
