FROM golang:1.24-alpine AS builder
ARG VERSION='0.1.0'
ARG AGENT_NAME='Weather Agent'
ARG AGENT_DESCRIPTION='An agent that provides weather information.'
WORKDIR /app
RUN apk add --no-cache curl
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X 'main.Version=${VERSION}' -X 'main.AgentName=${AGENT_NAME}' -X 'main.AgentDescription=${AGENT_DESCRIPTION}'" -o weather-agent .

FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache curl
COPY --from=builder /app/weather-agent .
COPY card.json /app/.well-known/agent.json
EXPOSE 8080
CMD ["./weather-agent"]
