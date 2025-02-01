package tests

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/inference-gateway/inference-gateway/tests/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestStreamTokens(t *testing.T) {
	tests := []struct {
		name              string
		provider          string
		model             string
		mockResponse      string
		messages          []providers.Message
		expectedResponses []providers.GenerateResponse
		testCancel        bool
		expectError       bool
	}{
		{
			name:     "Ollama successful response",
			provider: providers.OllamaID,
			mockResponse: `{"model":"phi3:3.8b","created_at":"2025-01-30T19:15:55.740038795Z","response":"how","done":false}
{"model":"phi3:3.8b","created_at":"2025-01-30T19:15:55.740038795Z","response":" are","done":false}
{"model":"phi3:3.8b","created_at":"2025-01-30T19:15:55.740038795Z","response":" you?","done":false}
{"model":"phi3:3.8b","created_at":"2025-01-31T16:47:15.158460303Z","response":"","done":true,"done_reason":"stop","context":[32006,29871],"total_duration":14508007757,"load_duration":4831567378,"prompt_eval_count":34,"prompt_eval_duration":1266000000,"eval_count":108,"eval_duration":8405000000}

`,
			messages: []providers.Message{
				{Role: "user", Content: "Hello"},
			},
			expectedResponses: []providers.GenerateResponse{
				{
					Provider: "Ollama",
					Response: providers.ResponseTokens{
						Content: "how",
						Model:   "phi3:3.8b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: "Ollama",
					Response: providers.ResponseTokens{
						Content: " are",
						Model:   "phi3:3.8b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: "Ollama",
					Response: providers.ResponseTokens{
						Content: " you?",
						Model:   "phi3:3.8b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
			},
			testCancel:  false,
			expectError: false,
		},
		{
			name:     "Context cancellation",
			provider: providers.OllamaID,
			mockResponse: `{"model":"phi3:3.8b","created_at":"2025-01-30T19:15:55.740038795Z","response":" are","done":false}
                        `,
			messages: []providers.Message{
				{Role: "user", Content: "Hello"},
			},
			testCancel:  true,
			expectError: false,
		},
		{
			name:     "Groq successful response",
			provider: providers.GroqID,
			model:    "deepseek-r1-distill-llama-70b",
			mockResponse: `
data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"role":"assistant","content":""},"logprobs":null,"finish_reason":null}],"x_groq":{"id":"req_***"}}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":"\\u003cthink\\u003e"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":"\\n\\n"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":"\\u003c/think\\u003e"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":"\\n\\n"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":"Hello"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":"!"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":" How"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":" can"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":" I"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":" assist"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":" you"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":" today"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":"?"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346484,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{"content":" 😊"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-***","object":"chat.completion.chunk","created":1738346485,"model":"deepseek-r1-distill-llama-70b","system_fingerprint":"fp_***","choices":[{"index":0,"delta":{},"logprobs":null,"finish_reason":"stop"}],"x_groq":{"id":"req_***","usage":{"queue_time":0.027146241,"prompt_tokens":10,"prompt_time":0.003496928,"completion_tokens":16,"completion_time":0.058181818,"total_tokens":26,"total_time":0.061678746}}}

data: [DONE]

`,
			messages: []providers.Message{
				{Role: "user", Content: "Hi"},
			},
			expectedResponses: []providers.GenerateResponse{
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: "",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventMessageStart,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: "\\u003cthink\\u003e",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: "\\n\\n",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: "\\u003c/think\\u003e",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: "\\n\\n",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: "Hello",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: "!",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: " How",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: " can",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: " I",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: " assist",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: " you",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: " today",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: "?",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: " 😊",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.GroqDisplayName,
					Response: providers.ResponseTokens{
						Content: "",
						Model:   "deepseek-r1-distill-llama-70b",
						Role:    "assistant",
					},
					EventType: providers.EventStreamEnd,
				},
			},
			testCancel:  false,
			expectError: false,
		},
		{
			name:     "Cohere successful response",
			provider: providers.CohereID,
			mockResponse: `

event: message-start
data: {"id":"***","type":"message-start","delta":{"message":{"role":"assistant","content":[],"tool_plan":"","tool_calls":[],"citations":[]}}}

event: content-start
data: {"type":"content-start","index":0,"delta":{"message":{"content":{"type":"text","text":""}}}}

event: content-delta
data: {"type":"content-delta","index":0,"delta":{"message":{"content":{"text":"Hello"}}}}

event: content-delta
data: {"type":"content-delta","index":0,"delta":{"message":{"content":{"text":"oooo"}}}}

event: content-end
data: {"type":"content-end","index":0}

event: message-end
data:  {"type":"message-end","delta":{"finish_reason":"COMPLETE","usage":{"billed_units":{"input_tokens":18,"output_tokens":55},"tokens":{"input_tokens":27,"output_tokens":55}}}}

        `,
			messages: []providers.Message{
				{Role: "user", Content: "Hello"},
			},
			expectedResponses: []providers.GenerateResponse{
				{
					Provider: providers.CohereDisplayName,
					Response: providers.ResponseTokens{
						Content: "",
						Model:   "N/A",
						Role:    "assistant",
					},
					EventType: providers.EventMessageStart,
				},
				{
					Provider: providers.CohereDisplayName,
					Response: providers.ResponseTokens{
						Content: "",
						Model:   "N/A",
						Role:    "assistant",
					},
					EventType: providers.EventContentStart,
				},
				{
					Provider: providers.CohereDisplayName,
					Response: providers.ResponseTokens{
						Content: "Hello",
						Model:   "N/A",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.CohereDisplayName,
					Response: providers.ResponseTokens{
						Content: "oooo",
						Model:   "N/A",
						Role:    "assistant",
					},
					EventType: providers.EventContentDelta,
				},
				{
					Provider: providers.CohereDisplayName,
					Response: providers.ResponseTokens{
						Content: "",
						Model:   "N/A",
						Role:    "assistant",
					},
					EventType: providers.EventContentEnd,
				},
				{
					Provider: providers.CohereDisplayName,
					Response: providers.ResponseTokens{
						Content: "",
						Model:   "N/A",
						Role:    "assistant",
					},
					EventType: providers.EventMessageEnd,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLogger := mocks.NewMockLogger(ctrl)
			mockClient := mocks.NewMockClient(ctrl)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mockClient.
				EXPECT().
				Do(gomock.Any()).
				Return(&http.Response{
					Body:       io.NopCloser(strings.NewReader(tt.mockResponse)),
					StatusCode: http.StatusOK,
				}, nil)

			var cfg *providers.Config
			switch tt.provider {
			case providers.OllamaID:
				cfg = &providers.Config{
					ID:   providers.OllamaID,
					Name: "Ollama",
					URL:  "http://test.local",
					Endpoints: providers.Endpoints{
						Generate: "/api/generate",
						List:     "/api/tags",
					},
					AuthType: providers.AuthTypeNone,
				}
			case providers.GroqID:
				cfg = &providers.Config{
					ID:   providers.GroqID,
					Name: "Groq",
					URL:  "http://test.local",
					Endpoints: providers.Endpoints{
						Generate: "/api/generate",
						List:     "/api/tags",
					},
					AuthType: providers.AuthTypeBearer,
					Token:    "test-token",
				}
			case providers.CohereID:
				cfg = &providers.Config{
					ID:   providers.CohereID,
					Name: "Cohere",
					URL:  "http://test.local",
					Endpoints: providers.Endpoints{
						Generate: "/api/generate",
						List:     "/api/tags",
					},
					AuthType: providers.AuthTypeBearer,
					Token:    "test-token",
				}
			default:
				cfg = &providers.Config{
					ID:   tt.provider,
					Name: "Default",
					URL:  "http://test.local",
					Endpoints: providers.Endpoints{
						Generate: "/api/generate",
						List:     "/api/tags",
					},
					AuthType: providers.AuthTypeNone,
					Token:    "test-token",
				}
			}

			providersRegistry := providers.NewProviderRegistry(
				map[string]*providers.Config{
					cfg.ID: cfg,
				},
				mockLogger,
			)

			var mc providers.Client = mockClient
			provider, err := providersRegistry.BuildProvider(cfg.ID, mc)
			assert.NoError(t, err)

			ch, err := provider.StreamTokens(ctx, tt.model, tt.messages)
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, ch)

			if !tt.testCancel {
				var responses []providers.GenerateResponse
				for resp := range ch {
					responses = append(responses, resp)
				}
				assert.Equal(t, tt.expectedResponses, responses)
			} else {
				cancel()
				_, ok := <-ch
				assert.False(t, ok, "channel should be closed after cancellation")
			}
		})
	}
}
