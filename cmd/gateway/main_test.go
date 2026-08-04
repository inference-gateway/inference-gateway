package main

import "testing"

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
