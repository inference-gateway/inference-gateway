package guardrails_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	guardrails "github.com/inference-gateway/inference-gateway/internal/guardrails"
	logger "github.com/inference-gateway/inference-gateway/logger"
)

const (
	toolName   = "mcp_time_get_time"
	toolArgs   = `{"timezone":"UTC"}`
	toolOutput = `{"content":[{"type":"text","text":"12:00"}]}`
	outputMark = "12:00"
	blockedMsg = "tool output refused"
	policyFile = "tool_output.rego"
	outputOnly = `package guardrails

main = {"action": "block", "message": "` + blockedMsg + `"} if {
	input.method == "TOOL_CALL"
	input.phase == "tool_output"
	contains(input.request.body, "` + outputMark + `")
}
`
)

// TestEvaluateToolCall_ToolOutputBody asserts the tool_output phase sees the
// tool output as input.request.body. The arguments never carry the marker, so
// a tool_args evaluation of the same policy must pass.
func TestEvaluateToolCall_ToolOutputBody(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, policyFile), []byte(outputOnly), 0o600))

	ctx := context.Background()
	evaluator, err := guardrails.NewEvaluator(ctx, dir)
	require.NoError(t, err)
	log := logger.NewNoopLogger()

	require.NoError(t, guardrails.EvaluateToolCall(ctx, evaluator, nil, log, guardrails.FailModeClosed, toolName, toolArgs, guardrails.PhaseToolArgs))

	err = guardrails.EvaluateToolCall(ctx, evaluator, nil, log, guardrails.FailModeClosed, toolName, toolOutput, guardrails.PhaseToolOutput)

	var blocked *guardrails.BlockedError
	require.ErrorAs(t, err, &blocked)
	assert.Equal(t, guardrails.PhaseToolOutput, blocked.Phase)
	assert.Equal(t, blockedMsg, blocked.Message)
}

// TestEvaluateToolCall_NilEvaluator asserts guardrails being disabled is not a
// block: the gateway passes a nil evaluator when GUARDRAILS_ENABLED is false.
func TestEvaluateToolCall_NilEvaluator(t *testing.T) {
	assert.NoError(t, guardrails.EvaluateToolCall(context.Background(), nil, nil, logger.NewNoopLogger(), guardrails.FailModeClosed, toolName, toolArgs, guardrails.PhaseToolArgs))
}
