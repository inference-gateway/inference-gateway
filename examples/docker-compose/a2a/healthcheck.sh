#!/bin/sh

echo "Checking health of all A2A agents..."

check_agent_health() {
    local agent_name="$1"
    local agent_url="$2"
    
    echo "Checking $agent_name at $agent_url..."
    if curl -f -s --max-time 5 "$agent_url" >/dev/null 2>&1; then
        echo "✓ $agent_name is healthy"
        return 0
    else
        echo "✗ $agent_name is unhealthy"
        return 1
    fi
}

all_healthy=true

check_agent_health "Hello World Agent" "http://helloworld-agent:8080/health" || all_healthy=false
check_agent_health "Calculator Agent" "http://calculator-agent:8080/health" || all_healthy=false
check_agent_health "Weather Agent" "http://weather-agent:8080/health" || all_healthy=false
check_agent_health "Google Calendar Agent" "http://google-calendar-agent:8080/health" || all_healthy=false

if [ "$all_healthy" = "true" ]; then
    echo "All A2A agents are healthy!"
    exit 0
else
    echo "One or more A2A agents are unhealthy"
    exit 1
fi
