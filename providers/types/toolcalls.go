package types

import (
	"encoding/json"
	"strings"
)

// AccumulateStreamingToolCalls reconstructs complete tool calls from an SSE
// stream body by accumulating the per-chunk deltas, indexed by tool call
// position. Tool calls that never received a function name are dropped.
func AccumulateStreamingToolCalls(body string) []ChatCompletionMessageToolCall {
	accumulated := make(map[int]*ChatCompletionMessageToolCall)

	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimSpace(line)
		data, found := strings.CutPrefix(line, "data: ")
		if !found {
			data = line
		}
		if data == "" || data == "[DONE]" {
			continue
		}

		var chunk CreateChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.ToolCalls == nil {
			continue
		}

		for _, delta := range *chunk.Choices[0].Delta.ToolCalls {
			toolCall, exists := accumulated[delta.Index]
			if !exists {
				toolCall = &ChatCompletionMessageToolCall{Type: Function}
				accumulated[delta.Index] = toolCall
			}

			if delta.ID != nil {
				toolCall.ID = *delta.ID
			}
			if delta.Type != nil {
				toolCall.Type = ChatCompletionToolType(*delta.Type)
			}
			if delta.Function != nil {
				if delta.Function.Name != "" {
					toolCall.Function.Name = delta.Function.Name
				}
				if delta.Function.Arguments != "" {
					toolCall.Function.Arguments += delta.Function.Arguments
				}
			}
		}
	}

	var toolCalls []ChatCompletionMessageToolCall
	for i := range len(accumulated) {
		if toolCall, exists := accumulated[i]; exists && toolCall.Function.Name != "" {
			toolCalls = append(toolCalls, *toolCall)
		}
	}
	return toolCalls
}
