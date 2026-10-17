package main

import (
	"bytes"
	"strings"
	"testing"
)

// invoke runs one CLI argument vector through the argv seam and returns the
// exit code plus both output channels as strings.
func invoke(args ...string) (code int, stdout, stderr string) {
	var out, err bytes.Buffer
	code = run(args, &out, &err)
	return code, out.String(), err.String()
}

// TestBareInvocationIsInvalidUsage covers T2's baseline: bare `bs` is invalid
// usage. The CLI must write a readable error to stderr, write nothing to
// stdout, and exit 1.
func TestBareInvocationIsInvalidUsage(t *testing.T) {
	code, stdout, stderr := invoke()

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Error("stderr is empty, want a readable error")
	}
}

// TestRootHelp covers T3/R2: `bs --help` exits 0, writes human-readable text
// naming both resource groups to stdout, and writes nothing to stderr.
func TestRootHelp(t *testing.T) {
	code, stdout, stderr := invoke("--help")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	for _, want := range []string{"incidents", "monitors"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\nstdout:\n%s", want, stdout)
		}
	}
	if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Errorf("stdout looks like JSON, want human-readable text:\n%s", stdout)
	}
}

// TestResourceHelp covers T4/R3: each resource help lists exactly its own
// commands and exits 0 with empty stderr.
func TestResourceHelp(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wants   []string
		notWant []string
	}{
		{
			name:  "incidents",
			args:  []string{"incidents", "--help"},
			wants: []string{"list", "get", "timeline"},
		},
		{
			name:    "monitors",
			args:    []string{"monitors", "--help"},
			wants:   []string{"list", "get"},
			notWant: []string{"timeline"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := invoke(tc.args...)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
			for _, want := range tc.wants {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout missing %q\nstdout:\n%s", want, stdout)
				}
			}
			for _, bad := range tc.notWant {
				if strings.Contains(stdout, bad) {
					t.Errorf("stdout unexpectedly contains %q\nstdout:\n%s", bad, stdout)
				}
			}
		})
	}
}
