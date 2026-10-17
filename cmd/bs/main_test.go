package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestBareInvocationIsInvalidUsage covers T2's baseline: bare `bs` is invalid
// usage. The CLI must write a readable error to stderr, write nothing to
// stdout, and exit 1.
func TestBareInvocationIsInvalidUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if strings.TrimSpace(stderr.String()) == "" {
		t.Error("stderr is empty, want a readable error")
	}
}
