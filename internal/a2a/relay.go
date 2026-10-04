package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"sync"

	types "github.com/inference-gateway/adk/types"
)

// JSON-RPC 2.0 error codes the relay answers with.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

const (
	protocolBindingJSONRPC = "JSONRPC"
	taskIDSeparator        = ":"

	// Keys of the params and results the relay rewrites or reads.
	keyID               = "id"
	keyTaskID           = "taskId"
	keyReferenceTaskIDs = "referenceTaskIds"
	keyStatus           = "status"
	keyTenant           = "tenant"
	keyMessage          = "message"
	keyMetadata         = "metadata"
	keyAgent            = "agent"
	keyTasks            = "tasks"
	keyTotalSize        = "totalSize"
	keyPageSize         = "pageSize"
	keyNextPageToken    = "nextPageToken"
)

// Error is a JSON-RPC error the relay produced itself; upstream results are
// passed through untouched apart from the task id prefix.
type Error = types.JSONRPCError

// IsStreamingMethod reports whether the method answers with an SSE stream.
func IsStreamingMethod(method string) bool {
	switch types.A2AMethod(method) {
	case types.A2AMethodSendStreamingMessage, types.A2AMethodSubscribeToTask:
		return true
	}
	return false
}

// rootIDIsTask lists the methods whose top-level params.id is a task id; for
// the push notification config methods it is the config id instead.
func rootIDIsTask(method types.A2AMethod) bool {
	switch method {
	case types.A2AMethodGetTask, types.A2AMethodCancelTask, types.A2AMethodSubscribeToTask:
		return true
	}
	return false
}

// Call relays one non-streaming method to the agent the request names and
// returns the upstream result with task ids prefixed <alias>:. ListTasks
// without an agent hint fans out to every agent and merges.
func (r *Registry) Call(ctx context.Context, method types.A2AMethod, params types.Struct) (alias string, result any, rpcErr *Error) {
	alias, params, rpcErr = r.resolve(method, params)
	if rpcErr != nil {
		return "", nil, rpcErr
	}
	if alias == "" {
		result, rpcErr = r.listTasksFanOut(ctx, params)
		return "", result, rpcErr
	}

	entry, _ := r.lookup(alias)
	callCtx, cancel := r.callContext(ctx)
	defer cancel()

	switch method {
	case types.A2AMethodSendMessage:
		result, rpcErr = call(callCtx, alias, params, entry.client.SendTask)
	case types.A2AMethodGetTask:
		result, rpcErr = call(callCtx, alias, params, entry.client.GetTask)
	case types.A2AMethodCancelTask:
		result, rpcErr = call(callCtx, alias, params, entry.client.CancelTask)
	case types.A2AMethodListTasks:
		result, rpcErr = call(callCtx, alias, params, entry.client.ListTasks)
	case types.A2AMethodCreateTaskPushNotificationConfig:
		result, rpcErr = call(callCtx, alias, params, entry.client.SetTaskPushNotificationConfig)
	case types.A2AMethodGetTaskPushNotificationConfig:
		result, rpcErr = call(callCtx, alias, params, entry.client.GetTaskPushNotificationConfig)
	case types.A2AMethodListTaskPushNotificationConfigs:
		result, rpcErr = call(callCtx, alias, params, entry.client.ListTaskPushNotificationConfig)
	case types.A2AMethodDeleteTaskPushNotificationConfig:
		result, rpcErr = call(callCtx, alias, params, entry.client.DeleteTaskPushNotificationConfig)
	case types.A2AMethodGetExtendedAgentCard:
		result, rpcErr = call(callCtx, alias, params, entry.client.GetAuthenticatedExtendedCard)
	default:
		return alias, nil, &Error{Code: CodeMethodNotFound, Message: "method not found: " + string(method)}
	}
	return alias, result, rpcErr
}

// Stream opens the upstream SSE stream for SendStreamingMessage or
// SubscribeToTask. Every event's result has its task ids prefixed; the channel
// closes when the upstream closes or ctx is cancelled.
func (r *Registry) Stream(ctx context.Context, method types.A2AMethod, params types.Struct) (alias string, events <-chan any, rpcErr *Error) {
	alias, params, rpcErr = r.resolve(method, params)
	if rpcErr != nil {
		return "", nil, rpcErr
	}
	if alias == "" {
		return "", nil, r.missingAgentHint()
	}
	entry, _ := r.lookup(alias)

	var upstream <-chan types.JSONRPCSuccessResponse
	switch method {
	case types.A2AMethodSendStreamingMessage:
		upstream, rpcErr = stream(ctx, alias, params, entry.client.SendTaskStreaming)
	case types.A2AMethodSubscribeToTask:
		upstream, rpcErr = stream(ctx, alias, params, entry.client.ResubscribeTask)
	default:
		return alias, nil, &Error{Code: CodeMethodNotFound, Message: "method not found: " + string(method)}
	}
	if rpcErr != nil {
		return alias, nil, rpcErr
	}

	out := make(chan any)
	go func() {
		defer close(out)
		for event := range upstream {
			if event.Result == nil {
				r.logger.Warn("a2a stream event dropped: no result", "agent", alias, "method", string(method))
				continue
			}
			result := decodeResult(event.Result)
			rewriteTaskIDs(result, false, prefixer(alias))
			select {
			case out <- result:
			case <-ctx.Done():
				return
			}
		}
	}()
	return alias, out, nil
}

// resolve picks the agent a request addresses and returns the params ready to
// forward: tenant removed and task id prefixes stripped. The hints are, in
// order, params.tenant, params.message.metadata.agent and the <alias>: prefix
// of any task id; when several are present they must agree. An empty alias
// with a nil error means no hint was given.
func (r *Registry) resolve(method types.A2AMethod, params types.Struct) (string, types.Struct, *Error) {
	forwarded := make(types.Struct, len(params))
	maps.Copy(forwarded, params)

	hints := make([]string, 0)
	if tenant, _ := forwarded[keyTenant].(string); tenant != "" {
		hints = append(hints, tenant)
	}
	delete(forwarded, keyTenant)
	if agent := messageAgentHint(forwarded); agent != "" {
		hints = append(hints, agent)
	}

	stripped := make(map[string]struct{})
	rewriteTaskIDs(forwarded, rootIDIsTask(method), r.stripper(stripped))
	for alias := range stripped {
		hints = append(hints, alias)
	}

	alias := ""
	for _, hint := range hints {
		if alias != "" && hint != alias {
			return "", nil, &Error{Code: CodeInvalidParams, Message: fmt.Sprintf("conflicting agent hints %q and %q", alias, hint)}
		}
		alias = hint
	}
	if alias == "" {
		if method == types.A2AMethodListTasks {
			return "", forwarded, nil
		}
		return "", nil, r.missingAgentHint()
	}
	if _, ok := r.lookup(alias); !ok {
		return "", nil, &Error{Code: CodeInvalidParams, Message: fmt.Sprintf("unknown agent %q; known agents: %s", alias, joinAliases(r.aliases))}
	}
	return alias, forwarded, nil
}

func (r *Registry) missingAgentHint() *Error {
	return &Error{
		Code:    CodeInvalidParams,
		Message: "no agent named: set params.tenant or message.metadata.agent to one of " + joinAliases(r.aliases) + ", or use a task id issued by the gateway",
	}
}

func messageAgentHint(params types.Struct) string {
	message, _ := params[keyMessage].(map[string]any)
	metadata, _ := message[keyMetadata].(map[string]any)
	agent, _ := metadata[keyAgent].(string)
	return agent
}

// stripper removes the <alias>: prefix from task ids whose alias is registered
// and records every alias it saw; an unprefixed id passes through untouched.
func (r *Registry) stripper(seen map[string]struct{}) func(string) string {
	return func(id string) string {
		alias, rest, ok := strings.Cut(id, taskIDSeparator)
		if !ok {
			return id
		}
		if _, known := r.lookup(alias); !known {
			return id
		}
		seen[alias] = struct{}{}
		return rest
	}
}

func prefixer(alias string) func(string) string {
	return func(id string) string {
		return alias + taskIDSeparator + id
	}
}

// rewriteTaskIDs applies fn to every task id in an A2A params or result object:
// taskId and referenceTaskIds wherever they appear, the id of any object that
// also carries a status (a Task), and the root id when rootIDIsTask.
func rewriteTaskIDs(node any, rootIDIsTask bool, fn func(string) string) {
	switch value := node.(type) {
	case map[string]any:
		rewriteObjectTaskIDs(value, rootIDIsTask, fn)
	case []any:
		for _, child := range value {
			rewriteTaskIDs(child, false, fn)
		}
	}
}

func rewriteObjectTaskIDs(object map[string]any, rootIDIsTask bool, fn func(string) string) {
	_, isTask := object[keyStatus]
	for key, child := range object {
		switch {
		case key == keyTaskID, key == keyID && (isTask || rootIDIsTask):
			if id, ok := child.(string); ok {
				object[key] = fn(id)
			}
		case key == keyReferenceTaskIDs:
			if ids, ok := child.([]any); ok {
				for i, id := range ids {
					if s, ok := id.(string); ok {
						ids[i] = fn(s)
					}
				}
			}
		default:
			rewriteTaskIDs(child, false, fn)
		}
	}
}

// call decodes params into the ADK request type, invokes the agent and
// returns its result with task ids prefixed. A params shape the type rejects
// is -32602; any upstream failure is -32603 carrying the ADK error text.
func call[T any](ctx context.Context, alias string, params types.Struct, fn func(context.Context, T) (*types.JSONRPCSuccessResponse, error)) (any, *Error) {
	typed, rpcErr := convert[T](params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	resp, err := fn(ctx, typed)
	if err != nil {
		return nil, upstreamError(alias, err)
	}
	result := decodeResult(resp.Result)
	rewriteTaskIDs(result, false, prefixer(alias))
	return result, nil
}

func stream[T any](ctx context.Context, alias string, params types.Struct, fn func(context.Context, T) (<-chan types.JSONRPCSuccessResponse, error)) (<-chan types.JSONRPCSuccessResponse, *Error) {
	typed, rpcErr := convert[T](params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	events, err := fn(ctx, typed)
	if err != nil {
		return nil, upstreamError(alias, err)
	}
	return events, nil
}

func convert[T any](params types.Struct) (T, *Error) {
	var typed T
	raw, err := json.Marshal(params)
	if err != nil {
		return typed, &Error{Code: CodeInvalidParams, Message: "invalid params: " + err.Error()}
	}
	if err := json.Unmarshal(raw, &typed); err != nil {
		return typed, &Error{Code: CodeInvalidParams, Message: "invalid params: " + err.Error()}
	}
	return typed, nil
}

// upstreamError wraps whatever the ADK client reports, which includes the
// agent's own JSON-RPC error text, as an internal error naming the agent.
// ponytail: the upstream code is flattened into the message because the ADK
// client returns a plain error; surface it once the client exposes a typed one.
// The same gap drops mid-stream JSON-RPC errors in Stream, which the client
// decodes into a result-only envelope.
func upstreamError(alias string, err error) *Error {
	return &Error{Code: CodeInternalError, Message: fmt.Sprintf("agent %q failed: %v", alias, err)}
}

// decodeResult turns the raw result the ADK client hands back into a generic
// JSON value so task ids can be rewritten in place.
func decodeResult(result types.Value) any {
	raw, ok := result.(json.RawMessage)
	if !ok {
		return result
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return result
	}
	return decoded
}

type fanOutResult struct {
	result any
	rpcErr *Error
}

// listTasksFanOut asks every agent for its tasks concurrently and merges the
// pages into one ListTasksResponse; an agent that fails is skipped.
// ponytail: pagination is per agent, so the merged nextPageToken is empty;
// add cursor fan-in if a client needs to page across agents.
func (r *Registry) listTasksFanOut(ctx context.Context, params types.Struct) (any, *Error) {
	results := make([]fanOutResult, len(r.aliases))
	var wg sync.WaitGroup
	for i, alias := range r.aliases {
		wg.Go(func() {
			entry, _ := r.lookup(alias)
			callCtx, cancel := r.callContext(ctx)
			defer cancel()
			result, rpcErr := call(callCtx, alias, params, entry.client.ListTasks)
			results[i] = fanOutResult{result: result, rpcErr: rpcErr}
		})
	}
	wg.Wait()

	tasks := make([]any, 0)
	totalSize, pageSize, succeeded := 0.0, 0.0, 0
	for i, outcome := range results {
		if outcome.rpcErr != nil {
			r.logger.Warn("a2a ListTasks fan-out skipped an agent", "agent", r.aliases[i], "error", outcome.rpcErr.Message)
			continue
		}
		succeeded++
		page, _ := outcome.result.(map[string]any)
		if pageTasks, ok := page[keyTasks].([]any); ok {
			tasks = append(tasks, pageTasks...)
		}
		size, _ := page[keyTotalSize].(float64)
		totalSize += size
		size, _ = page[keyPageSize].(float64)
		pageSize = max(pageSize, size)
	}
	if succeeded == 0 && len(r.aliases) > 0 {
		return nil, &Error{Code: CodeInternalError, Message: "every agent failed to list tasks"}
	}
	return map[string]any{
		keyTasks:         tasks,
		keyTotalSize:     totalSize,
		keyPageSize:      pageSize,
		keyNextPageToken: "",
	}, nil
}

func joinAliases(aliases []string) string {
	if len(aliases) == 0 {
		return "(none configured)"
	}
	return strings.Join(aliases, ", ")
}
