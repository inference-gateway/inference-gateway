# Google Calendar Agent

A comprehensive A2A (Agent-to-Agent) calendar management agent that integrates with Google Calendar API to provide calendar operations through natural language.

If you want to test it with the real Google Calendar API, I'd suggest you to create a new calendar for testing purposes, as the agent will modify events in the calendar it is configured to use.

## Features

The Google Calendar Agent supports the following operations:

### 📅 List Events

- View upcoming events for specific time periods
- Filter by today, tomorrow, this week, or next week
- Get detailed event information including time, location, and description

**Examples:**

- "Show me my events today"
- "What's on my calendar this week?"
- "List my meetings tomorrow"

### ➕ Create Events

- Schedule new meetings and appointments
- Parse natural language for event details
- Extract time, date, location, and attendee information

**Examples:**

- "Schedule a meeting with John at 2pm tomorrow"
- "Create a dentist appointment on Friday at 10am"
- "Book lunch with Sarah next Tuesday at 12:30pm"

### ✏️ Update Events

- Modify existing calendar events
- Change time, location, or other event details
- Reschedule meetings

**Examples:**

- "Move my 3pm meeting to 4pm"
- "Change the location of tomorrow's standup to conference room B"
- "Update the title of my 2pm appointment"

### ❌ Delete Events

- Cancel meetings and appointments
- Remove events from calendar

**Examples:**

- "Cancel my 4pm meeting"
- "Delete tomorrow's dentist appointment"
- "Remove the lunch meeting with Sarah"

## Configuration

### Google Calendar API Setup

1. **Create a Google Cloud Project**

   - Go to the [Google Cloud Console](https://console.cloud.google.com/)
   - Create a new project or select an existing one

2. **Enable the Calendar API**

   - Navigate to "APIs & Services" > "Library"
   - Search for "Google Calendar API"
   - Click "Enable"

3. **Create Service Account Credentials**

   - Go to "APIs & Services" > "Credentials"
   - Click "Create Credentials" > "Service Account"
   - Fill in the service account details
   - Download the JSON key file as `credentials.json`

4. **Share Calendar with Service Account**
   - Open Google Calendar
   - Go to calendar settings
   - Share your calendar with the service account email
   - Grant "Make changes and manage sharing" permission

### Environment Variables

- `GOOGLE_CALENDAR_SA_JSON`: The SA in a JSON format
- `GOOGLE_CALENDAR_ID`: Google Calendar ID to use (defaults to "primary")

## Running the Agent

### Using Docker Compose

The agent is included in the A2A docker-compose setup:

```bash
cd /workspaces/inference-gateway/examples/docker-compose/a2a
docker-compose up google-calendar-agent
```

### Running Locally

1. **Install Dependencies**

   ```bash
   go mod download
   ```

2. **Set up Credentials**

   ```bash
   # Place your credentials.json file in the agent directory
   cp /path/to/your/credentials.json .
   ```

3. **Run the Agent**
   ```bash
   go run main.go
   ```

The agent will start on port 8082.

## API Endpoints

### Health Check

```
GET /health
```

### Agent Information

```
GET /.well-known/agent.json
```

### A2A Protocol

```
POST /a2a
```

## Testing

You can test the agent using curl or any HTTP client:

```bash
# Check agent info
curl http://localhost:8082/.well-known/agent.json

# Send a calendar request
curl -X POST http://localhost:8082/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "params": {
      "message": {
        "role": "user",
        "parts": [
          {
            "type": "text",
            "text": "Show me my events today"
          }
        ]
      }
    },
    "id": "test-1"
  }'
```

## Architecture

The agent follows these design principles:

- **Interface-based Design**: Uses `CalendarService` interface for easy testing and mocking
- **Table-driven Testing**: Supports comprehensive test coverage
- **Early Returns**: Simplified control flow with early returns
- **Type Safety**: Strong typing throughout the codebase
- **Structured Logging**: Comprehensive logging with zap logger

## Error Handling

The agent handles various error scenarios:

- Invalid Google Calendar API credentials
- Network connectivity issues
- Malformed A2A requests
- Calendar permission errors
- Event parsing failures

In demo mode (when Google Calendar API is not available), the agent returns mock data for demonstration purposes.

## Natural Language Processing

The agent includes sophisticated natural language processing to:

- Detect intent (list, create, update, delete)
- Extract event details from text
- Parse dates and times in various formats
- Identify locations and attendees
- Handle common calendar terminology

## Security Considerations

- Service account credentials should be kept secure
- Calendar sharing should be limited to necessary permissions
- API rate limits should be considered for high-volume usage
- Input validation prevents malicious requests

## Contributing

When contributing to this agent:

1. Follow the coding standards in the custom instructions
2. Add comprehensive tests for new features
3. Update documentation for API changes
4. Run `task lint` and `task test` before committing
5. Use structured logging for debugging information

## License

This agent is part of the Inference Gateway project and follows the same license terms.
