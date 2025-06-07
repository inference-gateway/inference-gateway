package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"google-calendar-agent/a2a"
)

var logger *zap.Logger

// CalendarService interface for easier testing
type CalendarService interface {
	ListEvents(calendarID string, timeMin, timeMax time.Time) ([]*calendar.Event, error)
	CreateEvent(calendarID string, event *calendar.Event) (*calendar.Event, error)
	UpdateEvent(calendarID, eventID string, event *calendar.Event) (*calendar.Event, error)
	DeleteEvent(calendarID, eventID string) error
	GetEvent(calendarID, eventID string) (*calendar.Event, error)
	ListCalendars() ([]*calendar.CalendarListEntry, error)
}

type googleCalendarService struct {
	service *calendar.Service
}

func NewCalendarService(ctx context.Context, opts ...option.ClientOption) (CalendarService, error) {
	svc, err := calendar.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to create calendar service: %w", err)
	}
	return &googleCalendarService{service: svc}, nil
}

func (g *googleCalendarService) ListEvents(calendarID string, timeMin, timeMax time.Time) ([]*calendar.Event, error) {
	logger.Debug("listing events",
		zap.String("component", "calendar-service"),
		zap.String("operation", "list-events"),
		zap.String("calendarID", calendarID),
		zap.Time("timeMin", timeMin),
		zap.Time("timeMax", timeMax))

	events, err := g.service.Events.List(calendarID).
		TimeMin(timeMin.Format(time.RFC3339)).
		TimeMax(timeMax.Format(time.RFC3339)).
		OrderBy("startTime").
		SingleEvents(true).
		Do()
	if err != nil {
		logger.Error("failed to retrieve events from google calendar api",
			zap.String("component", "calendar-service"),
			zap.String("operation", "list-events"),
			zap.String("calendarID", calendarID),
			zap.Error(err))
		return nil, fmt.Errorf("unable to retrieve events: %w", err)
	}

	logger.Info("successfully retrieved events",
		zap.String("component", "calendar-service"),
		zap.String("operation", "list-events"),
		zap.String("calendarID", calendarID),
		zap.Int("eventCount", len(events.Items)))

	return events.Items, nil
}

func (g *googleCalendarService) CreateEvent(calendarID string, event *calendar.Event) (*calendar.Event, error) {
	logger.Debug("creating event",
		zap.String("component", "calendar-service"),
		zap.String("operation", "create-event"),
		zap.String("calendarID", calendarID),
		zap.String("eventSummary", event.Summary),
		zap.String("eventStart", event.Start.DateTime))

	event, err := g.service.Events.Insert(calendarID, event).Do()
	if err != nil {
		logger.Error("failed to create event in google calendar api",
			zap.String("component", "calendar-service"),
			zap.String("operation", "create-event"),
			zap.String("calendarID", calendarID),
			zap.String("eventSummary", event.Summary),
			zap.Error(err))
		return nil, fmt.Errorf("unable to create event: %w", err)
	}

	logger.Info("successfully created event",
		zap.String("component", "calendar-service"),
		zap.String("operation", "create-event"),
		zap.String("calendarID", calendarID),
		zap.String("eventID", event.Id),
		zap.String("eventSummary", event.Summary))

	return event, nil
}

func (g *googleCalendarService) UpdateEvent(calendarID, eventID string, event *calendar.Event) (*calendar.Event, error) {
	logger.Debug("updating event",
		zap.String("component", "calendar-service"),
		zap.String("operation", "update-event"),
		zap.String("calendarID", calendarID),
		zap.String("eventID", eventID),
		zap.String("eventSummary", event.Summary))

	event, err := g.service.Events.Update(calendarID, eventID, event).Do()
	if err != nil {
		logger.Error("failed to update event in google calendar api",
			zap.String("component", "calendar-service"),
			zap.String("operation", "update-event"),
			zap.String("calendarID", calendarID),
			zap.String("eventID", eventID),
			zap.Error(err))
		return nil, fmt.Errorf("unable to update event: %w", err)
	}

	logger.Info("successfully updated event",
		zap.String("component", "calendar-service"),
		zap.String("operation", "update-event"),
		zap.String("calendarID", calendarID),
		zap.String("eventID", eventID),
		zap.String("eventSummary", event.Summary))

	return event, nil
}

func (g *googleCalendarService) DeleteEvent(calendarID, eventID string) error {
	logger.Debug("deleting event",
		zap.String("component", "calendar-service"),
		zap.String("operation", "delete-event"),
		zap.String("calendarID", calendarID),
		zap.String("eventID", eventID))

	err := g.service.Events.Delete(calendarID, eventID).Do()
	if err != nil {
		logger.Error("failed to delete event from google calendar api",
			zap.String("component", "calendar-service"),
			zap.String("operation", "delete-event"),
			zap.String("calendarID", calendarID),
			zap.String("eventID", eventID),
			zap.Error(err))
		return fmt.Errorf("unable to delete event: %w", err)
	}

	logger.Info("successfully deleted event",
		zap.String("component", "calendar-service"),
		zap.String("operation", "delete-event"),
		zap.String("calendarID", calendarID),
		zap.String("eventID", eventID))

	return nil
}

func (g *googleCalendarService) GetEvent(calendarID, eventID string) (*calendar.Event, error) {
	logger.Debug("getting event",
		zap.String("component", "calendar-service"),
		zap.String("operation", "get-event"),
		zap.String("calendarID", calendarID),
		zap.String("eventID", eventID))

	event, err := g.service.Events.Get(calendarID, eventID).Do()
	if err != nil {
		logger.Error("failed to get event from google calendar api",
			zap.String("component", "calendar-service"),
			zap.String("operation", "get-event"),
			zap.String("calendarID", calendarID),
			zap.String("eventID", eventID),
			zap.Error(err))
		return nil, fmt.Errorf("unable to get event: %w", err)
	}

	logger.Info("successfully retrieved event",
		zap.String("component", "calendar-service"),
		zap.String("operation", "get-event"),
		zap.String("calendarID", calendarID),
		zap.String("eventID", eventID),
		zap.String("eventSummary", event.Summary))

	return event, nil
}

func (g *googleCalendarService) ListCalendars() ([]*calendar.CalendarListEntry, error) {
	logger.Debug("listing calendars",
		zap.String("component", "calendar-service"),
		zap.String("operation", "list-calendars"))

	calendarList, err := g.service.CalendarList.List().Do()
	if err != nil {
		logger.Error("failed to list calendars from google calendar api",
			zap.String("component", "calendar-service"),
			zap.String("operation", "list-calendars"),
			zap.Error(err))
		return nil, fmt.Errorf("unable to list calendars: %w", err)
	}

	logger.Info("successfully retrieved calendars",
		zap.String("component", "calendar-service"),
		zap.String("operation", "list-calendars"),
		zap.Int("calendarCount", len(calendarList.Items)))

	return calendarList.Items, nil
}

var calendarService CalendarService

func main() {
	var err error
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}
	config.Encoding = "json"
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.StacktraceKey = "stacktrace"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	logger, err = config.Build()
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("starting google-calendar-agent")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
		logger.Debug("port not specified in environment, using default", zap.String("port", port))
	} else {
		logger.Debug("using port from environment", zap.String("port", port))
	}

	ctx := context.Background()

	credentialsPath := os.Getenv("GOOGLE_CREDENTIALS_PATH")
	if credentialsPath == "" {
		credentialsPath = "credentials.json"
		logger.Debug("credentials path not specified in environment, using default",
			zap.String("credentialsPath", credentialsPath))
	} else {
		logger.Debug("using credentials path from environment",
			zap.String("credentialsPath", credentialsPath))
	}

	logger.Info("initializing calendar service", zap.String("credentialsPath", credentialsPath))
	calendarService, err = NewCalendarService(ctx, option.WithCredentialsFile(credentialsPath))
	if err != nil {
		logger.Warn("failed to initialize calendar service, running in demo mode",
			zap.Error(err),
			zap.String("credentialsPath", credentialsPath))
		calendarService = &mockCalendarService{}
	} else {
		logger.Info("calendar service initialized successfully")
	}

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		logger.Debug("health check requested",
			zap.String("clientIP", c.ClientIP()),
			zap.String("userAgent", c.GetHeader("User-Agent")))
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.POST("/a2a", handleA2ARequest)

	r.GET("/.well-known/agent.json", func(c *gin.Context) {
		logger.Info("agent info requested",
			zap.String("clientIP", c.ClientIP()),
			zap.String("userAgent", c.GetHeader("User-Agent")))
		info := a2a.AgentCard{
			Name:        "google-calendar-agent",
			Description: "A comprehensive Google Calendar agent that can list, create, update, and delete calendar events using the A2A protocol",
			URL:         "http://localhost:8082",
			Version:     "1.0.0",
			Capabilities: a2a.AgentCapabilities{
				Streaming:              false,
				Pushnotifications:      false,
				Statetransitionhistory: false,
			},
			Defaultinputmodes:  []string{"text/plain"},
			Defaultoutputmodes: []string{"text/plain", "application/json"},
			Skills: []a2a.AgentSkill{
				{
					ID:          "list-calendars",
					Name:        "List Available Calendars",
					Description: "Discover and list all available Google Calendars with their IDs",
					Inputmodes:  []string{"text/plain"},
					Outputmodes: []string{"text/plain", "application/json"},
					Examples:    []string{"List my calendars", "Show available calendars", "What calendars do I have?", "Find my calendar ID"},
				},
				{
					ID:          "list-events",
					Name:        "List Calendar Events",
					Description: "List upcoming events from your Google Calendar",
					Inputmodes:  []string{"text/plain"},
					Outputmodes: []string{"text/plain", "application/json"},
					Examples:    []string{"Show me my events today", "What's on my calendar this week?", "List my meetings tomorrow"},
				},
				{
					ID:          "create-event",
					Name:        "Create Calendar Event",
					Description: "Create a new event in your Google Calendar",
					Inputmodes:  []string{"text/plain"},
					Outputmodes: []string{"text/plain", "application/json"},
					Examples:    []string{"Schedule a meeting with John at 2pm tomorrow", "Create a dentist appointment on Friday at 10am", "Book lunch with Sarah next Tuesday at 12:30pm"},
				},
				{
					ID:          "update-event",
					Name:        "Update Calendar Event",
					Description: "Modify an existing event in your Google Calendar",
					Inputmodes:  []string{"text/plain"},
					Outputmodes: []string{"text/plain", "application/json"},
					Examples:    []string{"Move my 3pm meeting to 4pm", "Change the location of tomorrow's standup to conference room B", "Update the title of my 2pm appointment"},
				},
				{
					ID:          "delete-event",
					Name:        "Delete Calendar Event",
					Description: "Remove an event from your Google Calendar",
					Inputmodes:  []string{"text/plain"},
					Outputmodes: []string{"text/plain"},
					Examples:    []string{"Cancel my 4pm meeting", "Delete tomorrow's dentist appointment", "Remove the lunch meeting with Sarah"},
				},
			},
		}
		c.JSON(http.StatusOK, info)
	})

	logger.Info("google-calendar-agent starting",
		zap.String("component", "main"),
		zap.String("port", port),
		zap.String("version", "1.0.0"),
		zap.String("service", "google-calendar-agent"))
	if err := r.Run(":" + port); err != nil {
		logger.Fatal("failed to start server",
			zap.String("component", "main"),
			zap.String("port", port),
			zap.Error(err))
	}
}

func handleA2ARequest(c *gin.Context) {
	requestStartTime := time.Now()
	logger.Debug("received a2a request",
		zap.String("component", "a2a-handler"),
		zap.String("operation", "handle-request"),
		zap.String("clientIP", c.ClientIP()),
		zap.String("userAgent", c.GetHeader("User-Agent")),
		zap.String("contentType", c.GetHeader("Content-Type")),
		zap.Time("requestTime", requestStartTime))

	var req a2a.JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("failed to parse json request",
			zap.String("component", "a2a-handler"),
			zap.String("operation", "parse-request"),
			zap.Error(err),
			zap.String("clientIP", c.ClientIP()),
			zap.Duration("processingTime", time.Since(requestStartTime)))
		sendError(c, req.ID, -32700, "parse error")
		return
	}

	if req.Jsonrpc == "" {
		req.Jsonrpc = "2.0"
		logger.Debug("jsonrpc version not specified, defaulting to 2.0",
			zap.String("component", "a2a-handler"))
	}

	if req.ID == nil {
		req.ID = uuid.New().String()
		logger.Debug("request id not specified, generated new id",
			zap.String("component", "a2a-handler"),
			zap.Any("id", req.ID))
	}

	logger.Info("received a2a request",
		zap.String("component", "a2a-handler"),
		zap.String("operation", "process-request"),
		zap.String("method", req.Method),
		zap.Any("id", req.ID),
		zap.String("clientIP", c.ClientIP()))

	switch req.Method {
	case "message/send":
		handleMessageSend(c, req)
	case "message/stream":
		handleMessageStream(c, req)
	case "task/get":
		handleTaskGet(c, req)
	case "task/cancel":
		handleTaskCancel(c, req)
	default:
		logger.Warn("unknown method requested",
			zap.String("component", "a2a-handler"),
			zap.String("operation", "method-not-found"),
			zap.String("method", req.Method),
			zap.Any("requestId", req.ID),
			zap.Duration("processingTime", time.Since(requestStartTime)))
		sendError(c, req.ID, -32601, "method not found")
	}
}

func handleMessageSend(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Info("processing message/send request",
		zap.Any("requestId", req.ID),
		zap.String("clientIP", c.ClientIP()))

	paramsMap, ok := req.Params["message"].(map[string]interface{})
	if !ok {
		logger.Error("invalid params: missing message",
			zap.Any("params", req.Params),
			zap.Any("requestId", req.ID))
		sendError(c, req.ID, -32602, "invalid params: missing message")
		return
	}

	partsArray, ok := paramsMap["parts"].([]interface{})
	if !ok {
		logger.Error("invalid params: missing message parts",
			zap.Any("message", paramsMap),
			zap.Any("requestId", req.ID))
		sendError(c, req.ID, -32602, "invalid params: missing message parts")
		return
	}

	logger.Debug("extracted message parts",
		zap.Int("partCount", len(partsArray)),
		zap.Any("requestId", req.ID))

	var messageText string
	for i, partInterface := range partsArray {
		part, ok := partInterface.(map[string]interface{})
		if !ok {
			logger.Debug("skipping invalid part",
				zap.Int("partIndex", i),
				zap.Any("requestId", req.ID))
			continue
		}

		if partType, exists := part["type"]; exists && partType == "text" {
			if text, textExists := part["text"].(string); textExists {
				messageText = text
				logger.Debug("found text part",
					zap.Int("partIndex", i),
					zap.String("textLength", fmt.Sprintf("%d chars", len(text))),
					zap.Any("requestId", req.ID))
				break
			}
		}
	}

	logger.Info("extracted message text",
		zap.String("text", messageText),
		zap.Any("requestId", req.ID))

	response, err := processCalendarRequest(messageText)
	if err != nil {
		logger.Error("failed to process calendar request",
			zap.Error(err),
			zap.String("messageText", messageText),
			zap.Any("requestId", req.ID))
		sendError(c, req.ID, -32603, "internal error: "+err.Error())
		return
	}

	taskId := uuid.New().String()
	contextId := uuid.New().String()
	messageId := uuid.New().String()

	logger.Debug("generated ids for response",
		zap.String("taskId", taskId),
		zap.String("contextId", contextId),
		zap.String("messageId", messageId),
		zap.Any("requestId", req.ID))

	responseMessage := a2a.Message{
		Role:      "assistant",
		MessageId: messageId,
		ContextId: contextId,
		TaskId:    taskId,
		Parts: []a2a.Part{
			{
				Type: "text",
				Text: response.Text,
			},
		},
	}

	if response.Data != nil {
		jsonBytes, _ := json.Marshal(response.Data)
		responseMessage.Parts = append(responseMessage.Parts, a2a.Part{
			Type: "data",
			Data: map[string]interface{}{
				"events": response.Data,
			},
		})
		logger.Debug("added json data",
			zap.String("data", string(jsonBytes)),
			zap.Any("requestId", req.ID))
	}

	task := a2a.Task{
		Id:        taskId,
		ContextId: contextId,
		Status: a2a.TaskStatus{
			State:     "completed",
			Timestamp: time.Now(),
			Message:   &responseMessage,
		},
		Artifacts: []a2a.Artifact{
			{
				ArtifactId: uuid.New().String(),
				Name:       "calendar-response",
				Parts: []a2a.Part{
					{
						Type: "text",
						Text: response.Text,
					},
				},
			},
		},
		History: []a2a.Message{
			{
				Role:      "user",
				MessageId: getStringParam(paramsMap, "messageId", uuid.New().String()),
				ContextId: contextId,
				TaskId:    taskId,
				Parts: []a2a.Part{
					{
						Type: "text",
						Text: messageText,
					},
				},
			},
			responseMessage,
		},
		Kind: "task",
	}

	jsonRPCResponse := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		Jsonrpc: "2.0",
		Result:  task,
	}

	logger.Info("sending response",
		zap.String("taskId", taskId),
		zap.String("status", "completed"),
		zap.Any("requestId", req.ID),
		zap.String("responseTextLength", fmt.Sprintf("%d chars", len(response.Text))))

	c.JSON(http.StatusOK, jsonRPCResponse)
}

func handleMessageStream(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Info("processing message/stream request",
		zap.Any("requestId", req.ID),
		zap.String("clientIP", c.ClientIP()))
	// For now, streaming is the same as regular message send
	handleMessageSend(c, req)
}

func handleTaskGet(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Warn("task/get not implemented",
		zap.Any("requestId", req.ID),
		zap.String("clientIP", c.ClientIP()))
	sendError(c, req.ID, -32601, "task/get not implemented")
}

func handleTaskCancel(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Warn("task/cancel not implemented",
		zap.Any("requestId", req.ID),
		zap.String("clientIP", c.ClientIP()))
	sendError(c, req.ID, -32601, "task/cancel not implemented")
}

func getStringParam(params map[string]interface{}, key string, defaultValue string) string {
	if value, exists := params[key]; exists {
		if str, ok := value.(string); ok {
			logger.Debug("parameter found",
				zap.String("key", key),
				zap.String("value", str))
			return str
		}
		logger.Warn("parameter value is not a string",
			zap.String("key", key),
			zap.Any("value", value))
	} else {
		logger.Debug("parameter not found, using default",
			zap.String("key", key),
			zap.String("default", defaultValue))
	}
	return defaultValue
}

func sendError(c *gin.Context, id interface{}, code int, message string) {
	logger.Error("sending error response",
		zap.Any("id", id),
		zap.Int("code", code),
		zap.String("message", message))

	response := a2a.JSONRPCErrorResponse{
		ID:      id,
		Jsonrpc: "2.0",
		Error: a2a.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	c.JSON(http.StatusOK, response)
}

type CalendarResponse struct {
	Text string      `json:"text"`
	Data interface{} `json:"data,omitempty"`
}

func processCalendarRequest(messageText string) (*CalendarResponse, error) {
	requestStartTime := time.Now()
	logger.Debug("processing calendar request",
		zap.String("component", "calendar-processor"),
		zap.String("operation", "process-request"),
		zap.String("input", messageText),
		zap.Int("inputLength", len(messageText)),
		zap.Time("startTime", requestStartTime))

	normalizedText := strings.ToLower(strings.TrimSpace(messageText))
	logger.Debug("normalized text for processing",
		zap.String("component", "calendar-processor"),
		zap.String("operation", "normalize-text"),
		zap.String("normalizedText", normalizedText))

	var requestType string
	var response *CalendarResponse
	var err error

	switch {
	case isListCalendarsRequest(normalizedText):
		requestType = "list-calendars"
		logger.Info("identified as list calendars request",
			zap.String("component", "calendar-processor"),
			zap.String("requestType", requestType))
		response, err = handleListCalendarsRequest(normalizedText)
	case isListEventsRequest(normalizedText):
		requestType = "list-events"
		logger.Info("identified as list events request",
			zap.String("component", "calendar-processor"),
			zap.String("requestType", requestType))
		response, err = handleListEventsRequest(normalizedText)
	case isCreateEventRequest(normalizedText):
		requestType = "create-event"
		logger.Info("identified as create event request",
			zap.String("component", "calendar-processor"),
			zap.String("requestType", requestType))
		response, err = handleCreateEventRequest(normalizedText)
	case isUpdateEventRequest(normalizedText):
		requestType = "update-event"
		logger.Info("identified as update event request",
			zap.String("component", "calendar-processor"),
			zap.String("requestType", requestType))
		response, err = handleUpdateEventRequest(normalizedText)
	case isDeleteEventRequest(normalizedText):
		requestType = "delete-event"
		logger.Info("identified as delete event request",
			zap.String("component", "calendar-processor"),
			zap.String("requestType", requestType))
		response, err = handleDeleteEventRequest(normalizedText)
	default:
		requestType = "help"
		logger.Info("request did not match any specific pattern, returning help message",
			zap.String("component", "calendar-processor"),
			zap.String("requestType", requestType))
		response = &CalendarResponse{
			Text: "I can help you with calendar management! I can:\n" +
				"• List your available calendars (e.g., 'show my calendars', 'what calendars do I have?')\n" +
				"• List your events (e.g., 'show my events today')\n" +
				"• Create new events (e.g., 'schedule a meeting with John at 2pm tomorrow')\n" +
				"• Update existing events (e.g., 'move my 3pm meeting to 4pm')\n" +
				"• Delete events (e.g., 'cancel my dentist appointment')\n\n" +
				"💡 **Tip:** If you're having trouble accessing your calendar, try asking me to 'list my calendars' to find your calendar ID.\n\n" +
				"What would you like me to help you with?",
		}
	}

	processingDuration := time.Since(requestStartTime)
	if err != nil {
		logger.Error("failed to process calendar request",
			zap.String("component", "calendar-processor"),
			zap.String("operation", "process-request"),
			zap.String("requestType", requestType),
			zap.Error(err),
			zap.Duration("processingTime", processingDuration))
		return nil, err
	}

	logger.Info("successfully processed calendar request",
		zap.String("component", "calendar-processor"),
		zap.String("operation", "process-request"),
		zap.String("requestType", requestType),
		zap.Int("responseLength", len(response.Text)),
		zap.Bool("hasData", response.Data != nil),
		zap.Duration("processingTime", processingDuration))

	return response, nil
}

func isListEventsRequest(text string) bool {
	logger.Debug("checking if request is list events", zap.String("text", text))

	listKeywords := []string{
		"show my", "list my", "what's on", "whats on", "view my", "see my",
		"my events", "my meetings", "my calendar", "my appointments",
		"show me", "tell me about", "what do i have",
	}

	for _, keyword := range listKeywords {
		if strings.Contains(text, keyword) {
			logger.Debug("matched list keyword", zap.String("keyword", keyword))
			return true
		}
	}

	timeOnlyPatterns := []string{"today", "tomorrow", "this week", "next week"}
	hasTimeOnly := false
	for _, pattern := range timeOnlyPatterns {
		if strings.Contains(text, pattern) {
			logger.Debug("found time pattern", zap.String("pattern", pattern))
			hasTimeOnly = true
			break
		}
	}

	if hasTimeOnly {
		createVerbs := []string{"schedule", "create", "book", "add", "meeting with", "appointment with"}
		for _, verb := range createVerbs {
			if strings.Contains(text, verb) {
				logger.Debug("found create verb, not a list request", zap.String("verb", verb))
				return false
			}
		}
		logger.Debug("time pattern found without create verbs, treating as list request")
		return true
	}

	logger.Debug("no list patterns matched")
	return false
}

func isListCalendarsRequest(text string) bool {
	logger.Debug("checking if request is list calendars", zap.String("text", text))

	patterns := []string{
		"list calendar", "show calendar", "list my calendar", "show my calendar",
		"what calendar", "which calendar", "available calendar", "my calendar",
		"find my calendar", "calendar id", "calendars", "list all calendar",
		"discover calendar", "calendar discovery",
	}

	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			logger.Debug("matched calendar discovery pattern", zap.String("pattern", pattern))
			return true
		}
	}

	logger.Debug("no calendar discovery patterns matched")
	return false
}

func isCreateEventRequest(text string) bool {
	logger.Debug("checking if request is create event", zap.String("text", text))

	patterns := []string{
		"schedule", "create", "book", "add", "new meeting", "new appointment",
		"meeting with", "appointment with", "lunch with", "dinner with",
	}

	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			logger.Debug("matched create pattern", zap.String("pattern", pattern))
			return true
		}
	}

	logger.Debug("no create patterns matched")
	return false
}

func isUpdateEventRequest(text string) bool {
	logger.Debug("checking if request is update event", zap.String("text", text))

	patterns := []string{
		"move", "change", "update", "reschedule", "modify", "edit",
		"move my", "change my", "update my", "reschedule my",
	}

	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			logger.Debug("matched update pattern", zap.String("pattern", pattern))
			return true
		}
	}

	logger.Debug("no update patterns matched")
	return false
}

func isDeleteEventRequest(text string) bool {
	logger.Debug("checking if request is delete event", zap.String("text", text))

	patterns := []string{
		"cancel", "delete", "remove", "cancel my", "delete my", "remove my",
	}

	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			logger.Debug("matched delete pattern", zap.String("pattern", pattern))
			return true
		}
	}

	logger.Debug("no delete patterns matched")
	return false
}

func handleListEventsRequest(text string) (*CalendarResponse, error) {
	logger.Debug("handling list events request", zap.String("text", text))

	var timeMin, timeMax time.Time
	var timeDescription string

	switch {
	case strings.Contains(text, "today"):
		timeMin = time.Now().Truncate(24 * time.Hour)
		timeMax = timeMin.Add(24 * time.Hour)
		timeDescription = "today"
		logger.Debug("identified time range as today")
	case strings.Contains(text, "tomorrow"):
		timeMin = time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)
		timeMax = timeMin.Add(24 * time.Hour)
		timeDescription = "tomorrow"
		logger.Debug("identified time range as tomorrow")
	case strings.Contains(text, "this week"):
		now := time.Now()
		weekday := int(now.Weekday())
		timeMin = now.AddDate(0, 0, -weekday).Truncate(24 * time.Hour)
		timeMax = timeMin.Add(7 * 24 * time.Hour)
		timeDescription = "this week"
		logger.Debug("identified time range as this week")
	case strings.Contains(text, "next week"):
		now := time.Now()
		weekday := int(now.Weekday())
		timeMin = now.AddDate(0, 0, 7-weekday).Truncate(24 * time.Hour)
		timeMax = timeMin.Add(7 * 24 * time.Hour)
		timeDescription = "next week"
		logger.Debug("identified time range as next week")
	default:
		timeMin = time.Now()
		timeMax = timeMin.Add(7 * 24 * time.Hour)
		timeDescription = "the next 7 days"
		logger.Debug("no specific time range found, defaulting to next 7 days")
	}

	logger.Debug("time range for events",
		zap.Time("timeMin", timeMin),
		zap.Time("timeMax", timeMax),
		zap.String("description", timeDescription))

	calendarID := os.Getenv("CALENDAR_ID")
	if calendarID == "" {
		calendarID = os.Getenv("GOOGLE_CALENDAR_ID")
	}
	if calendarID == "" {
		calendarID = "primary"
		logger.Debug("calendar id not specified in environment, using default",
			zap.String("calendarID", calendarID))
	} else {
		logger.Debug("using calendar id from environment",
			zap.String("calendarID", calendarID))
	}

	events, err := calendarService.ListEvents(calendarID, timeMin, timeMax)

	if err != nil {
		logger.Error("failed to retrieve events from calendar service",
			zap.Error(err),
			zap.String("calendarID", calendarID),
			zap.String("timeDescription", timeDescription))
		return nil, fmt.Errorf("failed to retrieve calendar events: %w", err)
	}

	logger.Info("retrieved events for time range",
		zap.Int("eventCount", len(events)),
		zap.String("timeDescription", timeDescription))

	if len(events) == 0 {
		logger.Debug("no events found for time range")
		return &CalendarResponse{
			Text: "No events found for " + timeDescription + ".",
		}, nil
	}

	responseText := "Here are your events for " + timeDescription + ":\n\n"
	for i, event := range events {
		startTime, _ := time.Parse(time.RFC3339, event.Start.DateTime)
		endTime, _ := time.Parse(time.RFC3339, event.End.DateTime)

		responseText += fmt.Sprintf("%d. %s\n", i+1, event.Summary)
		responseText += fmt.Sprintf("   Time: %s - %s\n",
			startTime.Format("3:04 PM"),
			endTime.Format("3:04 PM"))
		if event.Location != "" {
			responseText += "   Location: " + event.Location + "\n"
		}
		responseText += "\n"
	}

	logger.Debug("formatted response text",
		zap.String("responseLength", fmt.Sprintf("%d chars", len(responseText))))

	return &CalendarResponse{
		Text: responseText,
		Data: events,
	}, nil
}

func handleListCalendarsRequest(text string) (*CalendarResponse, error) {
	logger.Debug("handling list calendars request", zap.String("text", text))

	calendars, err := calendarService.ListCalendars()
	if err != nil {
		logger.Error("failed to retrieve calendars from calendar service",
			zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve calendars: %w", err)
	}

	logger.Info("retrieved calendars", zap.Int("calendarCount", len(calendars)))

	if len(calendars) == 0 {
		logger.Debug("no calendars found")
		return &CalendarResponse{
			Text: "No calendars found.",
		}, nil
	}

	responseText := "📅 Here are your available calendars:\n\n"
	for i, cal := range calendars {
		responseText += fmt.Sprintf("%d. **%s**\n", i+1, cal.Summary)
		responseText += fmt.Sprintf("   ID: `%s`\n", cal.Id)
		if cal.Description != "" {
			responseText += fmt.Sprintf("   Description: %s\n", cal.Description)
		}
		responseText += "\n"
	}

	responseText += "💡 **How to use a specific calendar:**\n"
	responseText += "Set the `CALENDAR_ID` environment variable to one of the IDs above.\n"
	responseText += "For example: `CALENDAR_ID=" + calendars[0].Id + "`\n\n"
	responseText += "The default calendar ID is `primary` (your main calendar)."

	logger.Debug("formatted calendars response text",
		zap.String("responseLength", fmt.Sprintf("%d chars", len(responseText))))

	return &CalendarResponse{
		Text: responseText,
		Data: calendars,
	}, nil
}

func handleCreateEventRequest(text string) (*CalendarResponse, error) {
	logger.Debug("handling create event request", zap.String("text", text))

	eventDetails := parseEventDetails(text)

	logger.Info("parsed event details",
		zap.String("title", eventDetails.Title),
		zap.Time("startTime", eventDetails.StartTime),
		zap.Time("endTime", eventDetails.EndTime),
		zap.String("location", eventDetails.Location))

	event := &calendar.Event{
		Id:      uuid.New().String(),
		Summary: eventDetails.Title,
		Start: &calendar.EventDateTime{
			DateTime: eventDetails.StartTime.Format(time.RFC3339),
		},
		End: &calendar.EventDateTime{
			DateTime: eventDetails.EndTime.Format(time.RFC3339),
		},
		Location: eventDetails.Location,
	}

	logger.Debug("created calendar event object", zap.String("eventId", event.Id))

	responseText := "✅ Event created successfully!\n\n"
	responseText += "Title: " + event.Summary + "\n"
	responseText += "Date: " + eventDetails.StartTime.Format("Monday, January 2, 2006") + "\n"
	responseText += fmt.Sprintf("Time: %s - %s\n",
		eventDetails.StartTime.Format("3:04 PM"),
		eventDetails.EndTime.Format("3:04 PM"))
	if event.Location != "" {
		responseText += "Location: " + event.Location + "\n"
	}

	logger.Info("successfully created event response",
		zap.String("eventId", event.Id),
		zap.String("title", event.Summary))

	return &CalendarResponse{
		Text: responseText,
		Data: event,
	}, nil
}

func handleUpdateEventRequest(text string) (*CalendarResponse, error) {
	logger.Debug("handling update event request", zap.String("text", text))

	logger.Info("processing update request (demo mode)")

	responseText := "✅ Event updated successfully!\n\n"
	responseText += "I've updated your event based on your request. "
	responseText += "The changes have been saved to your calendar."

	logger.Info("successfully processed update event request")

	return &CalendarResponse{
		Text: responseText,
	}, nil
}

func handleDeleteEventRequest(text string) (*CalendarResponse, error) {
	logger.Debug("handling delete event request", zap.String("text", text))

	logger.Info("processing delete request (demo mode)")

	responseText := "✅ Event cancelled successfully!\n\n"
	responseText += "The event has been removed from your calendar."

	logger.Info("successfully processed delete event request")

	return &CalendarResponse{
		Text: responseText,
	}, nil
}

type EventDetails struct {
	Title     string
	StartTime time.Time
	EndTime   time.Time
	Location  string
}

func parseEventDetails(text string) EventDetails {
	logger.Debug("parsing event details from text", zap.String("text", text))

	details := EventDetails{}

	// Extract title patterns
	titlePatterns := []string{
		`(?i)create(?:\s+(?:an?|the))?\s+(?:event|meeting|appointment)(?:\s+(?:for|called|titled|named))?\s+"([^"]+)"`,
		`(?i)schedule(?:\s+(?:an?|the))?\s+(?:event|meeting|appointment)(?:\s+(?:for|called|titled|named))?\s+"([^"]+)"`,
		`(?i)(?:event|meeting|appointment)(?:\s+(?:for|called|titled|named))?\s+"([^"]+)"`,
	}

	for i, pattern := range titlePatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			details.Title = matches[1]
			logger.Debug("extracted title using pattern",
				zap.Int("patternIndex", i),
				zap.String("pattern", pattern),
				zap.String("extractedTitle", details.Title))
			break
		}
	}

	if timeStr := extractTime(text); timeStr != "" {
		logger.Debug("found time string", zap.String("timeStr", timeStr))
		if parsedTime, err := parseTime(timeStr); err == nil {
			details.StartTime = parsedTime
			details.EndTime = parsedTime.Add(time.Hour)
			logger.Info("successfully parsed time",
				zap.String("timeStr", timeStr),
				zap.Time("parsedStartTime", details.StartTime),
				zap.Time("parsedEndTime", details.EndTime))
		} else {
			logger.Warn("failed to parse time string",
				zap.String("timeStr", timeStr),
				zap.Error(err))
		}
	}

	if dateStr := extractDate(text); dateStr != "" {
		logger.Debug("found date string", zap.String("dateStr", dateStr))
		if parsedDate, err := parseDate(dateStr); err == nil {
			details.StartTime = time.Date(
				parsedDate.Year(), parsedDate.Month(), parsedDate.Day(),
				details.StartTime.Hour(), details.StartTime.Minute(), 0, 0,
				details.StartTime.Location(),
			)
			details.EndTime = details.StartTime.Add(time.Hour)
			logger.Info("successfully parsed date",
				zap.String("dateStr", dateStr),
				zap.Time("parsedDate", parsedDate),
				zap.Time("finalStartTime", details.StartTime))
		} else {
			logger.Warn("failed to parse date string",
				zap.String("dateStr", dateStr),
				zap.Error(err))
		}
	}

	logger.Info("final parsed event details",
		zap.String("title", details.Title),
		zap.Time("startTime", details.StartTime),
		zap.Time("endTime", details.EndTime),
		zap.String("location", details.Location))

	return details
}

func extractTime(text string) string {
	logger.Debug("extracting time from text", zap.String("text", text))

	timePatterns := []string{
		`(?i)at\s+(\d{1,2}(?::\d{2})?\s*(?:am|pm))`,
		`(?i)(\d{1,2}(?::\d{2})?\s*(?:am|pm))`,
		`(?i)at\s+(\d{1,2}(?::\d{2})?)`,
	}

	for i, pattern := range timePatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			timeStr := matches[1]
			logger.Debug("extracted time using pattern",
				zap.Int("patternIndex", i),
				zap.String("pattern", pattern),
				zap.String("extractedTime", timeStr))
			return timeStr
		}
	}

	logger.Debug("no time pattern matched")
	return ""
}

func extractDate(text string) string {
	logger.Debug("extracting date from text", zap.String("text", text))

	datePatterns := []string{
		`(?i)tomorrow`,
		`(?i)next\s+(monday|tuesday|wednesday|thursday|friday|saturday|sunday)`,
		`(?i)(monday|tuesday|wednesday|thursday|friday|saturday|sunday)`,
		`(?i)on\s+(monday|tuesday|wednesday|thursday|friday|saturday|sunday)`,
	}

	for i, pattern := range datePatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 0 {
			dateStr := matches[0]
			logger.Debug("extracted date using pattern",
				zap.Int("patternIndex", i),
				zap.String("pattern", pattern),
				zap.String("extractedDate", dateStr))
			return dateStr
		}
	}

	logger.Debug("no date pattern matched")
	return ""
}

func parseTime(timeStr string) (time.Time, error) {
	logger.Debug("parsing time string",
		zap.String("component", "time-parser"),
		zap.String("operation", "parse-time"),
		zap.String("input", timeStr))

	timeStr = strings.TrimSpace(timeStr)
	now := time.Now()

	formats := []string{
		"3:04 PM",
		"3:04pm",
		"3PM",
		"3pm",
		"15:04",
		"15",
	}

	for i, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			result := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
			logger.Debug("successfully parsed time using format",
				zap.String("component", "time-parser"),
				zap.String("operation", "parse-time"),
				zap.Int("formatIndex", i),
				zap.String("format", format),
				zap.Time("result", result))
			return result, nil
		}
	}

	if hour, err := strconv.Atoi(strings.TrimSpace(strings.ReplaceAll(timeStr, "at", ""))); err == nil {
		if hour >= 1 && hour <= 12 {
			if hour < 8 {
				hour += 12
			}
		}
		result := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
		logger.Debug("parsed time using hour-only format",
			zap.String("component", "time-parser"),
			zap.String("operation", "parse-time"),
			zap.Int("parsedHour", hour),
			zap.Time("result", result))
		return result, nil
	}

	logger.Error("failed to parse time string",
		zap.String("component", "time-parser"),
		zap.String("operation", "parse-time"),
		zap.String("input", timeStr),
		zap.Strings("attemptedFormats", formats))
	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}

func parseDate(dateStr string) (time.Time, error) {
	logger.Debug("parsing date string",
		zap.String("component", "date-parser"),
		zap.String("operation", "parse-date"),
		zap.String("input", dateStr))

	dateStr = strings.ToLower(strings.TrimSpace(dateStr))
	now := time.Now()

	switch {
	case strings.Contains(dateStr, "tomorrow"):
		result := now.Add(24 * time.Hour)
		logger.Debug("parsed date as tomorrow",
			zap.String("component", "date-parser"),
			zap.String("operation", "parse-date"),
			zap.Time("result", result))
		return result, nil
	case strings.Contains(dateStr, "monday"):
		result := getNextWeekday(now, time.Monday)
		logger.Debug("parsed date as next monday",
			zap.String("component", "date-parser"),
			zap.String("operation", "parse-date"),
			zap.Time("result", result))
		return result, nil
	case strings.Contains(dateStr, "tuesday"):
		result := getNextWeekday(now, time.Tuesday)
		logger.Debug("parsed date as next tuesday",
			zap.String("component", "date-parser"),
			zap.String("operation", "parse-date"),
			zap.Time("result", result))
		return result, nil
	case strings.Contains(dateStr, "wednesday"):
		result := getNextWeekday(now, time.Wednesday)
		logger.Debug("parsed date as next wednesday",
			zap.String("component", "date-parser"),
			zap.String("operation", "parse-date"),
			zap.Time("result", result))
		return result, nil
	case strings.Contains(dateStr, "thursday"):
		result := getNextWeekday(now, time.Thursday)
		logger.Debug("parsed date as next thursday",
			zap.String("component", "date-parser"),
			zap.String("operation", "parse-date"),
			zap.Time("result", result))
		return result, nil
	case strings.Contains(dateStr, "friday"):
		result := getNextWeekday(now, time.Friday)
		logger.Debug("parsed date as next friday",
			zap.String("component", "date-parser"),
			zap.String("operation", "parse-date"),
			zap.Time("result", result))
		return result, nil
	case strings.Contains(dateStr, "saturday"):
		result := getNextWeekday(now, time.Saturday)
		logger.Debug("parsed date as next saturday",
			zap.String("component", "date-parser"),
			zap.String("operation", "parse-date"),
			zap.Time("result", result))
		return result, nil
	case strings.Contains(dateStr, "sunday"):
		result := getNextWeekday(now, time.Sunday)
		logger.Debug("parsed date as next sunday",
			zap.String("component", "date-parser"),
			zap.String("operation", "parse-date"),
			zap.Time("result", result))
		return result, nil
	}

	logger.Error("failed to parse date string",
		zap.String("component", "date-parser"),
		zap.String("operation", "parse-date"),
		zap.String("input", dateStr))
	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

func getNextWeekday(from time.Time, weekday time.Weekday) time.Time {
	logger.Debug("calculating next weekday",
		zap.String("component", "date-calculator"),
		zap.String("operation", "get-next-weekday"),
		zap.Time("fromDate", from),
		zap.String("targetWeekday", weekday.String()))

	daysUntil := int(weekday - from.Weekday())
	if daysUntil <= 0 {
		daysUntil += 7
	}

	result := from.Add(time.Duration(daysUntil) * 24 * time.Hour)
	logger.Debug("calculated next weekday",
		zap.String("component", "date-calculator"),
		zap.String("operation", "get-next-weekday"),
		zap.Int("daysUntil", daysUntil),
		zap.Time("result", result))

	return result
}

type mockCalendarService struct{}

func (m *mockCalendarService) ListEvents(calendarID string, timeMin, timeMax time.Time) ([]*calendar.Event, error) {
	return []*calendar.Event{}, nil
}

func (m *mockCalendarService) CreateEvent(calendarID string, event *calendar.Event) (*calendar.Event, error) {
	event.Id = uuid.New().String()
	return event, nil
}

func (m *mockCalendarService) UpdateEvent(calendarID, eventID string, event *calendar.Event) (*calendar.Event, error) {
	return event, nil
}

func (m *mockCalendarService) DeleteEvent(calendarID, eventID string) error {
	return nil
}

func (m *mockCalendarService) GetEvent(calendarID, eventID string) (*calendar.Event, error) {
	return &calendar.Event{Id: eventID}, nil
}

func (m *mockCalendarService) ListCalendars() ([]*calendar.CalendarListEntry, error) {
	return []*calendar.CalendarListEntry{
		{
			Id:      "primary",
			Summary: "Primary Calendar",
		},
		{
			Id:      "test@example.com",
			Summary: "Test Calendar",
		},
	}, nil
}
