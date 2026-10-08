package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApplyMiddlewaresRunsInOrder(t *testing.T) {
	var calls []string
	record := func(name string, serveNext bool) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, name)
				if serveNext {
					next.ServeHTTP(w, r)
				}
			})
		}
	}

	handler := applyMiddlewares(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls = append(calls, "mux") }),
		record("logger", true),
		record("auth", false),
		record("mcp", true),
	)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))

	want := []string{"logger", "auth"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	for i, name := range want {
		if calls[i] != name {
			t.Fatalf("calls = %v, want %v", calls, want)
		}
	}
}

func TestIsLoopbackHost(t *testing.T) {
	tests := map[string]bool{
		"127.0.0.1":    true,
		"127.0.0.53":   true,
		"::1":          true,
		"localhost":    true,
		"0.0.0.0":      false,
		"192.168.1.10": false,
		"::":           false,
		"":             false,
	}
	for host, want := range tests {
		if got := isLoopbackHost(host); got != want {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
