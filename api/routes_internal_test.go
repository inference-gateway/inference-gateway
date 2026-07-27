package api

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// stubProvider implements core.IProvider with configurable auth fields.
type stubProvider struct {
	authType     string
	token        string
	extraHeaders map[string][]string
}

func (s *stubProvider) GetID() *types.Provider               { return nil }
func (s *stubProvider) GetName() string                      { return "stub" }
func (s *stubProvider) GetURL() string                       { return "https://example.com" }
func (s *stubProvider) GetToken() string                     { return s.token }
func (s *stubProvider) GetAuthType() string                  { return s.authType }
func (s *stubProvider) GetExtraHeaders() map[string][]string { return s.extraHeaders }
func (s *stubProvider) ListModels(_ context.Context) (types.ListModelsResponse, error) {
	return types.ListModelsResponse{}, nil
}
func (s *stubProvider) ChatCompletions(_ context.Context, _ types.CreateChatCompletionRequest) (types.CreateChatCompletionResponse, error) {
	return types.CreateChatCompletionResponse{}, nil
}
func (s *stubProvider) StreamChatCompletions(_ context.Context, _ types.CreateChatCompletionRequest) (<-chan []byte, error) {
	return nil, nil
}
func (s *stubProvider) SupportsVision(_ context.Context, _ string) (bool, error) { return false, nil }

func TestApplyProviderAuth_StripsCallerAuthorization(t *testing.T) {
	tests := []struct {
		name           string
		authType       string
		token          string
		wantAuthHeader string // empty means header should be absent
		wantXAPIKey    string // empty means header should be absent
		wantQueryKey   string // empty means no ?key= in URL
	}{
		{
			name:           "bearer overwrites caller Authorization",
			authType:       constants.AuthTypeBearer,
			token:          "sk-provider",
			wantAuthHeader: "Bearer sk-provider",
		},
		{
			name:        "xheader strips caller Authorization, sets x-api-key",
			authType:    constants.AuthTypeXheader,
			token:       "sk-provider",
			wantXAPIKey: "sk-provider",
		},
		{
			name:         "query strips caller Authorization, sets ?key=",
			authType:     constants.AuthTypeQuery,
			token:        "sk-provider",
			wantQueryKey: "sk-provider",
		},
		{
			name:     "none strips caller Authorization, no credential remains",
			authType: constants.AuthTypeNone,
			token:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header: http.Header{"Authorization": []string{"Bearer caller-token"}},
				URL:    &url.URL{Path: "/v1/chat", RawQuery: "existing=param"},
			}

			provider := &stubProvider{authType: tt.authType, token: tt.token}

			if err := applyProviderAuth(req, provider); err != nil {
				t.Fatalf("applyProviderAuth() error = %v", err)
			}

			// Caller's original Authorization must never reach the upstream
			if v := req.Header.Get("Authorization"); v == "Bearer caller-token" {
				t.Errorf("caller Authorization header leaked through")
			}

			// Check expected auth header
			if tt.wantAuthHeader != "" {
				if v := req.Header.Get("Authorization"); v != tt.wantAuthHeader {
					t.Errorf("Authorization = %q, want %q", v, tt.wantAuthHeader)
				}
			} else {
				if v := req.Header.Get("Authorization"); v != "" {
					t.Errorf("unexpected Authorization header: %q", v)
				}
			}

			// Check x-api-key
			if tt.wantXAPIKey != "" {
				if v := req.Header.Get("x-api-key"); v != tt.wantXAPIKey {
					t.Errorf("x-api-key = %q, want %q", v, tt.wantXAPIKey)
				}
			} else {
				if v := req.Header.Get("x-api-key"); v != "" {
					t.Errorf("unexpected x-api-key: %q", v)
				}
			}

			// Check query param
			if tt.wantQueryKey != "" {
				if v := req.URL.Query().Get("key"); v != tt.wantQueryKey {
					t.Errorf("?key = %q, want %q", v, tt.wantQueryKey)
				}
			} else {
				if v := req.URL.Query().Get("key"); v != "" {
					t.Errorf("unexpected ?key: %q", v)
				}
			}

			// Existing query params must be preserved
			if v := req.URL.Query().Get("existing"); v != "param" {
				t.Errorf("existing query param lost: %q", v)
			}
		})
	}
}

func TestApplyProviderAuth_UnsupportedAuthType(t *testing.T) {
	req := &http.Request{
		Header: http.Header{"Authorization": []string{"Bearer caller-token"}},
		URL:    &url.URL{Path: "/v1/chat"},
	}
	provider := &stubProvider{authType: "unknown", token: "x"}

	if err := applyProviderAuth(req, provider); err == nil {
		t.Error("applyProviderAuth() expected error for unsupported auth type")
	}
}
