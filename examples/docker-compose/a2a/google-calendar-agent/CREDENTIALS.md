# Google Calendar Agent - Credentials Setup

This document explains how to set up Google Calendar API credentials for the Google Calendar agent.

## Quick Start (Demo Mode)

The agent works out of the box in **demo mode** without any credentials. It will use mock data to demonstrate functionality.

```bash
# No credentials needed - runs in demo mode
docker-compose up google-calendar-agent
```

## Production Setup with Google Calendar API

To integrate with real Google Calendar data, you need to set up Google Calendar API credentials.

### Step 1: Create a Google Cloud Project

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Google Calendar API:
   - Navigate to "APIs & Services" > "Library"
   - Search for "Google Calendar API"
   - Click "Enable"

### Step 2: Create Service Account Credentials

1. Go to "APIs & Services" > "Credentials"
2. Click "Create Credentials" > "Service Account"
3. Fill in the service account details:
   - **Name**: `calendar-agent-service`
   - **Description**: `Service account for Google Calendar Agent`
4. Click "Create and Continue"
5. Grant the service account access (optional for this step)
6. Click "Done"

### Step 3: Generate and Download Key

1. Click on the created service account
2. Go to the "Keys" tab
3. Click "Add Key" > "Create New Key"
4. Select "JSON" format
5. Click "Create" - this downloads the credentials file

### Step 4: Share Calendar with Service Account

1. Open [Google Calendar](https://calendar.google.com/)
2. On the left sidebar, find your calendar and click the three dots
3. Select "Settings and sharing"
4. Scroll down to "Share with specific people"
5. Click "Add people"
6. Enter the service account email (found in the JSON file as `client_email`)
7. Set permission to "Make changes to events"
8. Click "Send"

### Step 5: Configure Environment Variables

Set up the environment variables for your deployment:

#### Option A: Docker Compose

Create a `.env` file in the a2a directory:

```bash
# .env file
GOOGLE_CALENDAR_SA_JSON='{"type":"service_account","project_id":"your-project",...}'
GOOGLE_CALENDAR_ID=primary
```

#### Option B: Direct Environment Export

```bash
# Export the credentials JSON (replace with your actual credentials)
export GOOGLE_CALENDAR_SA_JSON='{"type":"service_account","project_id":"your-project","private_key_id":"...","private_key":"-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n","client_email":"calendar-agent-service@your-project.iam.gserviceaccount.com","client_id":"...","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token","auth_provider_x509_cert_url":"https://www.googleapis.com/oauth2/v1/certs","client_x509_cert_url":"https://www.googleapis.com/robot/v1/metadata/x509/calendar-agent-service%40your-project.iam.gserviceaccount.com"}'

# Optionally specify a specific calendar ID (defaults to "primary")
export GOOGLE_CALENDAR_ID=primary
```

#### Option C: Using File Path (for development)

```bash
# Save the JSON credentials to a file
echo '{"type":"service_account",...}' > /path/to/google-credentials.json

# Set the environment variable to the file path
export GOOGLE_CALENDAR_SA_JSON=$(cat /path/to/google-credentials.json)
```

### Step 6: Test the Connection

Start the agent and verify it can connect to Google Calendar:

```bash
# Start the agent
docker-compose up google-calendar-agent

# Test the health endpoint
curl http://localhost:8084/health

# Test the agent capabilities
curl http://localhost:8084/.well-known/agent.json

# Test with a simple request
curl -X POST http://localhost:8084 \
  -H "Content-Type: application/json" \
  -d '{
    "method": "message/send",
    "params": {
      "message": {
        "text": "show me my events today"
      }
    },
    "id": "test-1"
  }'
```

## Calendar ID Options

The `GOOGLE_CALENDAR_ID` environment variable accepts:

- `primary` - Your primary calendar (default)
- `calendar-id@group.calendar.google.com` - A specific calendar ID
- `email@gmail.com` - A specific user's primary calendar (if shared)

## Troubleshooting

### Common Issues

1. **"Calendar not found" error**

   - Make sure the service account has access to the calendar
   - Verify the GOOGLE_CALENDAR_ID is correct

2. **"Insufficient permissions" error**

   - Ensure the service account has "Make changes to events" permission
   - Check that the Google Calendar API is enabled

3. **"Invalid credentials" error**
   - Verify the JSON credentials are properly formatted
   - Ensure no extra spaces or newlines in the environment variable

### Debug Mode

Enable debug logging to troubleshoot issues:

```bash
# Add debug environment variable
export LOG_LEVEL=debug
docker-compose up google-calendar-agent
```

### Fallback to Demo Mode

If credentials are invalid or unavailable, the agent automatically falls back to demo mode with mock data. Check the logs for messages like:

```
INFO google-calendar-agent/main.go:XX using mock calendar service (demo mode)
```

## Security Best Practices

1. **Never commit credentials to version control**
2. **Use environment variables or secrets management**
3. **Rotate service account keys regularly**
4. **Grant minimal necessary permissions**
5. **Monitor service account usage in Google Cloud Console**

## Calendar Permissions

The service account needs the following calendar permissions:

- **View events**: To list and read calendar events
- **Edit events**: To create, update, and delete events
- **Manage sharing**: To access shared calendars (if needed)

These are typically granted with the "Make changes to events" permission level when sharing the calendar.
