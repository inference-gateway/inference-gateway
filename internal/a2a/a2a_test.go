package a2a_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	client "github.com/inference-gateway/adk/client"
	mocks "github.com/inference-gateway/adk/client/mocks"
	types "github.com/inference-gateway/adk/types"

	config "github.com/inference-gateway/inference-gateway/config"
	a2a "github.com/inference-gateway/inference-gateway/internal/a2a"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
)

const (
	researchAlias = "research"
	writerAlias   = "writer"
	researchURL   = "http://research-agent:8080"
	writerURL     = "http://writer-agent:8080"
	gatewayURL    = "http://gateway.example.com/a2a"
	gatewayVer    = "1.2.3"
	upstreamTask  = "task-1"
	prefixedTask  = researchAlias + ":" + upstreamTask
	configID      = "cfg-9"
	agentPassword = "s3cret"
	testTimeout   = 5 * time.Second
)

var errAgentDown = errors.New("connection refused")

func testConfig() config.A2AConfig {
	return config.A2AConfig{Enabled: true, ClientTimeout: testTimeout}
}

func fakeAgent(url string) *mocks.FakeA2AClient {
	fake := &mocks.FakeA2AClient{}
	fake.GetBaseURLReturns(url)
	return fake
}

func card(name string, streaming, push bool, skills ...string) *types.AgentCard {
	agentSkills := make([]types.AgentSkill, 0, len(skills))
	for _, id := range skills {
		agentSkills = append(agentSkills, types.AgentSkill{ID: id, Name: id, Description: id, Tags: []string{}})
	}
	return &types.AgentCard{
		Name:               name,
		Capabilities:       types.AgentCapabilities{Streaming: &streaming, PushNotifications: &push},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain", "application/json"},
		Skills:             agentSkills,
	}
}

func rawResult(t *testing.T, v any) *types.JSONRPCSuccessResponse {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return &types.JSONRPCSuccessResponse{JSONRPC: "2.0", ID: "upstream-id", Result: json.RawMessage(raw)}
}

func newRegistry(research, writer client.A2AClient) *a2a.Registry {
	return a2a.NewRegistry(testConfig(), logger.NewNoopLogger(), map[string]client.A2AClient{
		researchAlias: research,
		writerAlias:   writer,
	})
}

func TestParseAgents(t *testing.T) {
	specs, err := a2a.ParseAgents(researchAlias + "=" + researchURL + "," + writerURL)
	require.NoError(t, err)
	assert.Equal(t, researchAlias, specs[0].Alias)
	assert.Equal(t, "writer-agent", specs[1].Alias)

	_, err = a2a.ParseAgents("stdio:///agent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid a2a agent url")
}

func TestRefreshMarksReachability(t *testing.T) {
	research, writer := fakeAgent(researchURL), fakeAgent(writerURL)
	research.GetAgentCardReturns(card("Research", true, true, "search"), nil)
	writer.GetAgentCardReturns(nil, errAgentDown)

	registry := newRegistry(research, writer)
	registry.Refresh(context.Background())

	agents := registry.Agents()
	require.Len(t, agents, 2)
	assert.Equal(t, researchAlias, agents[0].Alias)
	assert.Equal(t, researchURL, agents[0].URL)
	assert.True(t, agents[0].Reachable)
	assert.NotNil(t, agents[0].Card)
	assert.False(t, agents[0].LastSeen.IsZero())
	assert.Equal(t, writerAlias, agents[1].Alias)
	assert.False(t, agents[1].Reachable)
	assert.Nil(t, agents[1].Card)
	assert.True(t, agents[1].LastSeen.IsZero())
}

func TestAgentsRedactsCredentials(t *testing.T) {
	registry := a2a.NewRegistry(testConfig(), logger.NewNoopLogger(), map[string]client.A2AClient{
		researchAlias: fakeAgent("https://user:" + agentPassword + "@research-agent:8443"),
	})

	agents := registry.Agents()
	require.Len(t, agents, 1)
	assert.Equal(t, "https://user:xxxxx@research-agent:8443", agents[0].URL)

	raw, err := json.Marshal(agents)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), agentPassword)
}

func TestCardWithoutAnyLoadedCardHasEmptyModeLists(t *testing.T) {
	registry := newRegistry(fakeAgent(researchURL), fakeAgent(writerURL))

	raw, err := json.Marshal(registry.Card(gatewayURL, gatewayVer))
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"defaultInputModes":[]`)
	assert.Contains(t, string(raw), `"defaultOutputModes":[]`)
}

func TestCardMergesAgents(t *testing.T) {
	research, writer := fakeAgent(researchURL), fakeAgent(writerURL)
	research.GetAgentCardReturns(card("Research", true, true, "search", "summarize"), nil)
	writer.GetAgentCardReturns(card("Writer", true, false, "draft"), nil)

	registry := newRegistry(research, writer)
	registry.Refresh(context.Background())
	merged := registry.Card(gatewayURL, gatewayVer)

	assert.Equal(t, gatewayVer, merged.Version)
	skillIDs := make([]string, 0, len(merged.Skills))
	for _, skill := range merged.Skills {
		skillIDs = append(skillIDs, skill.ID)
	}
	assert.Equal(t, []string{"research_search", "research_summarize", "writer_draft"}, skillIDs)
	assert.True(t, *merged.Capabilities.Streaming, "every agent streams")
	assert.False(t, *merged.Capabilities.PushNotifications, "writer has no push notifications")
	assert.Equal(t, []string{"text/plain"}, merged.DefaultInputModes)
	assert.Equal(t, []string{"application/json", "text/plain"}, merged.DefaultOutputModes)

	require.Len(t, merged.SupportedInterfaces, 3)
	assert.Nil(t, merged.SupportedInterfaces[0].Tenant)
	assert.Equal(t, gatewayURL, merged.SupportedInterfaces[0].URL)
	assert.Equal(t, researchAlias, *merged.SupportedInterfaces[1].Tenant)
	assert.Equal(t, writerAlias, *merged.SupportedInterfaces[2].Tenant)

	raw, err := json.Marshal(merged)
	require.NoError(t, err)
	var roundTrip types.AgentCard
	require.NoError(t, json.Unmarshal(raw, &roundTrip), "the merged card must stay a valid AgentCard")
}

func TestCardUnreachableAgentDisablesCapabilities(t *testing.T) {
	research, writer := fakeAgent(researchURL), fakeAgent(writerURL)
	research.GetAgentCardReturns(card("Research", true, true), nil)
	writer.GetAgentCardReturns(nil, errAgentDown)

	registry := newRegistry(research, writer)
	registry.Refresh(context.Background())
	merged := registry.Card(gatewayURL, gatewayVer)

	assert.False(t, *merged.Capabilities.Streaming)
	assert.False(t, *merged.Capabilities.PushNotifications)
}

func TestCallRoutesByHint(t *testing.T) {
	tests := []struct {
		name       string
		method     types.A2AMethod
		params     types.Struct
		wantCode   int
		wantMsg    string
		wantTaskID string
	}{
		{
			name:       "tenant names the agent and is not forwarded",
			method:     types.A2AMethodGetTask,
			params:     types.Struct{"id": upstreamTask, "tenant": researchAlias},
			wantTaskID: upstreamTask,
		},
		{
			name:       "prefixed task id names the agent and is stripped",
			method:     types.A2AMethodGetTask,
			params:     types.Struct{"id": prefixedTask},
			wantTaskID: upstreamTask,
		},
		{
			name:     "no hint lists the known agents",
			method:   types.A2AMethodGetTask,
			params:   types.Struct{"id": upstreamTask},
			wantCode: a2a.CodeInvalidParams,
			wantMsg:  researchAlias + ", " + writerAlias,
		},
		{
			name:     "unknown agent is invalid params",
			method:   types.A2AMethodGetTask,
			params:   types.Struct{"id": upstreamTask, "tenant": "nosuch"},
			wantCode: a2a.CodeInvalidParams,
			wantMsg:  `unknown agent "nosuch"`,
		},
		{
			name:     "conflicting hints are invalid params",
			method:   types.A2AMethodGetTask,
			params:   types.Struct{"id": prefixedTask, "tenant": writerAlias},
			wantCode: a2a.CodeInvalidParams,
			wantMsg:  "conflicting agent hints",
		},
		{
			name:     "params the ADK type rejects are invalid params",
			method:   types.A2AMethodGetTask,
			params:   types.Struct{"id": 42, "tenant": researchAlias},
			wantCode: a2a.CodeInvalidParams,
			wantMsg:  "invalid params",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			research := fakeAgent(researchURL)
			research.GetTaskStub = func(_ context.Context, req types.GetTaskRequest) (*types.JSONRPCSuccessResponse, error) {
				assert.Nil(t, req.Tenant, "tenant must not reach the agent")
				return rawResult(t, types.Task{ID: req.ID, Status: types.TaskStatus{State: "TASK_STATE_COMPLETED"}}), nil
			}
			registry := newRegistry(research, fakeAgent(writerURL))

			alias, result, rpcErr := registry.Call(context.Background(), tt.method, tt.params)

			if tt.wantCode != 0 {
				require.NotNil(t, rpcErr)
				assert.Equal(t, tt.wantCode, rpcErr.Code)
				assert.Contains(t, rpcErr.Message, tt.wantMsg)
				assert.Zero(t, research.GetTaskCallCount(), "a refused call never reaches an agent")
				return
			}
			require.Nil(t, rpcErr)
			assert.Equal(t, researchAlias, alias)
			_, forwarded := research.GetTaskArgsForCall(0)
			assert.Equal(t, tt.wantTaskID, forwarded.ID)
			task := result.(map[string]any)
			assert.Equal(t, prefixedTask, task["id"], "the task id leaves the gateway prefixed")
		})
	}
}

func TestCallSendMessageUsesMessageMetadata(t *testing.T) {
	research := fakeAgent(researchURL)
	research.SendTaskStub = func(_ context.Context, req types.SendMessageRequest) (*types.JSONRPCSuccessResponse, error) {
		assert.Equal(t, "ctx-1", *req.Message.ContextID)
		assert.Equal(t, []string{"older"}, req.Message.ReferenceTaskIDs, "referenced task ids are stripped")
		task := types.Task{ID: upstreamTask, Status: types.TaskStatus{State: "TASK_STATE_WORKING", Message: &types.Message{MessageID: "m2", TaskID: ptr(upstreamTask)}}}
		return rawResult(t, types.SendMessageResponse{Task: &task}), nil
	}
	registry := newRegistry(research, fakeAgent(writerURL))

	params := types.Struct{"message": map[string]any{
		"messageId":        "m1",
		"role":             "ROLE_USER",
		"contextId":        "ctx-1",
		"referenceTaskIds": []any{researchAlias + ":older"},
		"metadata":         map[string]any{"agent": researchAlias},
		"parts":            []any{map[string]any{"text": "hi"}},
	}}
	alias, result, rpcErr := registry.Call(context.Background(), types.A2AMethodSendMessage, params)

	require.Nil(t, rpcErr)
	assert.Equal(t, researchAlias, alias)
	task := result.(map[string]any)["task"].(map[string]any)
	assert.Equal(t, prefixedTask, task["id"])
	statusMessage := task["status"].(map[string]any)["message"].(map[string]any)
	assert.Equal(t, prefixedTask, statusMessage["taskId"], "nested task references are prefixed too")
}

func TestCallPushNotificationConfigKeepsConfigID(t *testing.T) {
	research := fakeAgent(researchURL)
	research.GetTaskPushNotificationConfigStub = func(_ context.Context, req types.GetTaskPushNotificationConfigRequest) (*types.JSONRPCSuccessResponse, error) {
		assert.Equal(t, configID, req.ID, "the config id is not a task id")
		assert.Equal(t, upstreamTask, req.TaskID)
		return rawResult(t, types.TaskPushNotificationConfig{ID: ptr(configID), TaskID: ptr(upstreamTask), URL: "https://client.example.com/hook"}), nil
	}
	registry := newRegistry(research, fakeAgent(writerURL))

	_, result, rpcErr := registry.Call(context.Background(), types.A2AMethodGetTaskPushNotificationConfig,
		types.Struct{"id": configID, "taskId": prefixedTask})

	require.Nil(t, rpcErr)
	cfg := result.(map[string]any)
	assert.Equal(t, configID, cfg["id"])
	assert.Equal(t, prefixedTask, cfg["taskId"])
}

func TestCallUpstreamFailureIsInternalError(t *testing.T) {
	research := fakeAgent(researchURL)
	research.CancelTaskReturns(nil, errAgentDown)
	registry := newRegistry(research, fakeAgent(writerURL))

	_, _, rpcErr := registry.Call(context.Background(), types.A2AMethodCancelTask, types.Struct{"id": prefixedTask})

	require.NotNil(t, rpcErr)
	assert.Equal(t, a2a.CodeInternalError, rpcErr.Code)
	assert.Contains(t, rpcErr.Message, researchAlias)
	assert.Contains(t, rpcErr.Message, errAgentDown.Error())
}

func TestCallGetExtendedAgentCardNeedsTenant(t *testing.T) {
	research := fakeAgent(researchURL)
	research.GetAuthenticatedExtendedCardReturns(rawResult(t, card("Research", true, true)), nil)
	registry := newRegistry(research, fakeAgent(writerURL))

	_, _, rpcErr := registry.Call(context.Background(), types.A2AMethodGetExtendedAgentCard, types.Struct{})
	require.NotNil(t, rpcErr)
	assert.Equal(t, a2a.CodeInvalidParams, rpcErr.Code)

	alias, result, rpcErr := registry.Call(context.Background(), types.A2AMethodGetExtendedAgentCard, types.Struct{"tenant": researchAlias})
	require.Nil(t, rpcErr)
	assert.Equal(t, researchAlias, alias)
	assert.Equal(t, "Research", result.(map[string]any)["name"])
}

func TestListTasksFansOutWithoutHint(t *testing.T) {
	research, writer := fakeAgent(researchURL), fakeAgent(writerURL)
	research.ListTasksReturns(rawResult(t, types.ListTasksResponse{
		Tasks:     []types.Task{{ID: "r1", Status: types.TaskStatus{State: "TASK_STATE_COMPLETED"}}},
		TotalSize: 1, PageSize: 50,
	}), nil)
	writer.ListTasksReturns(rawResult(t, types.ListTasksResponse{
		Tasks:     []types.Task{{ID: "w1", Status: types.TaskStatus{State: "TASK_STATE_WORKING"}}},
		TotalSize: 1, PageSize: 50,
	}), nil)
	registry := newRegistry(research, writer)

	alias, result, rpcErr := registry.Call(context.Background(), types.A2AMethodListTasks, types.Struct{})

	require.Nil(t, rpcErr)
	assert.Empty(t, alias)
	merged := result.(map[string]any)
	tasks := merged["tasks"].([]any)
	require.Len(t, tasks, 2)
	assert.Equal(t, researchAlias+":r1", tasks[0].(map[string]any)["id"])
	assert.Equal(t, writerAlias+":w1", tasks[1].(map[string]any)["id"])
	assert.Equal(t, 2.0, merged["totalSize"])
	assert.Equal(t, "", merged["nextPageToken"])
}

func TestListTasksFanOutSkipsFailedAgents(t *testing.T) {
	research, writer := fakeAgent(researchURL), fakeAgent(writerURL)
	research.ListTasksReturns(rawResult(t, types.ListTasksResponse{Tasks: []types.Task{{ID: "r1"}}, TotalSize: 1}), nil)
	writer.ListTasksReturns(nil, errAgentDown)
	registry := newRegistry(research, writer)

	_, result, rpcErr := registry.Call(context.Background(), types.A2AMethodListTasks, types.Struct{})
	require.Nil(t, rpcErr)
	assert.Len(t, result.(map[string]any)["tasks"], 1)

	research.ListTasksReturns(nil, errAgentDown)
	_, _, rpcErr = registry.Call(context.Background(), types.A2AMethodListTasks, types.Struct{})
	require.NotNil(t, rpcErr)
	assert.Equal(t, a2a.CodeInternalError, rpcErr.Code)
}

func TestListTasksWithTenantAsksOneAgent(t *testing.T) {
	research, writer := fakeAgent(researchURL), fakeAgent(writerURL)
	writer.ListTasksReturns(rawResult(t, types.ListTasksResponse{Tasks: []types.Task{{ID: "w1"}}}), nil)
	registry := newRegistry(research, writer)

	alias, result, rpcErr := registry.Call(context.Background(), types.A2AMethodListTasks, types.Struct{"tenant": writerAlias})

	require.Nil(t, rpcErr)
	assert.Equal(t, writerAlias, alias)
	assert.Zero(t, research.ListTasksCallCount())
	assert.Equal(t, writerAlias+":w1", result.(map[string]any)["tasks"].([]any)[0].(map[string]any)["id"])
}

func TestStreamPrefixesEveryEvent(t *testing.T) {
	research := fakeAgent(researchURL)
	research.SendTaskStreamingStub = func(_ context.Context, req types.SendMessageRequest) (<-chan types.JSONRPCSuccessResponse, error) {
		events := make(chan types.JSONRPCSuccessResponse, 2)
		events <- types.JSONRPCSuccessResponse{Result: map[string]any{"task": map[string]any{"id": upstreamTask, "status": map[string]any{"state": "TASK_STATE_SUBMITTED"}}}}
		events <- types.JSONRPCSuccessResponse{Result: map[string]any{"statusUpdate": map[string]any{"taskId": upstreamTask, "contextId": "ctx-1"}}}
		close(events)
		return events, nil
	}
	registry := newRegistry(research, fakeAgent(writerURL))

	alias, events, rpcErr := registry.Stream(context.Background(), types.A2AMethodSendStreamingMessage,
		types.Struct{"tenant": researchAlias, "message": map[string]any{"messageId": "m1", "role": "ROLE_USER", "parts": []any{}}})

	require.Nil(t, rpcErr)
	assert.Equal(t, researchAlias, alias)
	first := (<-events).(map[string]any)["task"].(map[string]any)
	assert.Equal(t, prefixedTask, first["id"])
	second := (<-events).(map[string]any)["statusUpdate"].(map[string]any)
	assert.Equal(t, prefixedTask, second["taskId"])
	_, open := <-events
	assert.False(t, open, "the relay closes when the upstream closes")
}

func TestStreamSubscribeStripsPrefix(t *testing.T) {
	research := fakeAgent(researchURL)
	research.ResubscribeTaskStub = func(_ context.Context, req types.SubscribeToTaskRequest) (<-chan types.JSONRPCSuccessResponse, error) {
		assert.Equal(t, upstreamTask, req.ID)
		events := make(chan types.JSONRPCSuccessResponse)
		close(events)
		return events, nil
	}
	registry := newRegistry(research, fakeAgent(writerURL))

	_, events, rpcErr := registry.Stream(context.Background(), types.A2AMethodSubscribeToTask, types.Struct{"id": prefixedTask})
	require.Nil(t, rpcErr)
	_, open := <-events
	assert.False(t, open)

	research.ResubscribeTaskReturns(nil, errAgentDown)
	_, _, rpcErr = registry.Stream(context.Background(), types.A2AMethodSubscribeToTask, types.Struct{"id": prefixedTask})
	require.NotNil(t, rpcErr)
	assert.Equal(t, a2a.CodeInternalError, rpcErr.Code)
}

func TestIsStreamingMethod(t *testing.T) {
	assert.True(t, a2a.IsStreamingMethod(string(types.A2AMethodSendStreamingMessage)))
	assert.True(t, a2a.IsStreamingMethod(string(types.A2AMethodSubscribeToTask)))
	assert.False(t, a2a.IsStreamingMethod(string(types.A2AMethodSendMessage)))
}

func ptr(s string) *string { return &s }
