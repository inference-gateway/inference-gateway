package main

import "testing"

func TestSandboxedStaysInBaseDir(t *testing.T) {
	for path, want := range map[string]string{
		"notes.txt":                 baseDir + "/notes.txt",
		"/etc/passwd":               baseDir + "/etc/passwd",
		"../../etc/passwd":          baseDir + "/etc/passwd",
		"../mcp-files-other/secret": baseDir + "/mcp-files-other/secret",
	} {
		if got := sandboxed(path); got != want {
			t.Errorf("sandboxed(%q) = %q, want %q", path, got, want)
		}
	}
}
