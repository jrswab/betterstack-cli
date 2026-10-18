package main

import (
	"strings"
	"testing"
	"time"
)

// TestParseIncidentsGetArgs covers A1 (spec R1.1, R1.2, R7.1): the incidents
// parser mirrors monitors get, accepting exactly one positional target and an
// optional --timeout in any position, and rejects every invalid vector before
// any request. No server is involved.
func TestParseIncidentsGetArgs(t *testing.T) {
	accepted := []struct {
		name    string
		args    []string
		target  string
		timeout time.Duration
	}{
		{"bare id", []string{"123"}, "123", defaultTimeout},
		{"url target", []string{"https://incidents.betterstack.com/team/t1/incidents/9"}, "https://incidents.betterstack.com/team/t1/incidents/9", defaultTimeout},
		{"timeout before target", []string{"--timeout", "60s", "123"}, "123", time.Minute},
		{"timeout after target", []string{"123", "--timeout", "250ms"}, "123", 250 * time.Millisecond},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			target, timeout, err := parseIncidentsGetArgs(tc.args)
			if err != nil {
				t.Fatalf("parseIncidentsGetArgs(%q) error = %v", tc.args, err)
			}
			if target != tc.target {
				t.Errorf("target = %q, want %q", target, tc.target)
			}
			if timeout != tc.timeout {
				t.Errorf("timeout = %v, want %v", timeout, tc.timeout)
			}
		})
	}

	rejected := []struct {
		name string
		args []string
	}{
		{"missing target", nil},
		{"too many positionals", []string{"1", "2"}},
		{"timeout missing value", []string{"123", "--timeout"}},
		{"timeout unparseable", []string{"123", "--timeout", "soon"}},
		{"timeout zero", []string{"123", "--timeout", "0s"}},
		{"timeout negative", []string{"123", "--timeout", "-1s"}},
		{"unknown long option", []string{"123", "--all"}},
		{"unknown short option", []string{"-h"}},
		{"help after target", []string{"123", "--help"}},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			target, timeout, err := parseIncidentsGetArgs(tc.args)
			if err == nil {
				t.Fatalf("parseIncidentsGetArgs(%q) = (%q, %v, nil), want an error", tc.args, target, timeout)
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Error("error message is empty, want a readable problem")
			}
		})
	}
}

// TestResolveIncidentTarget covers A1 (spec R2.1-R2.3): only the observed
// dashboard URL form resolves to its terminal ASCII-digit segment; every other
// scheme-ful form is rejected locally, while scheme-less targets stay opaque.
func TestResolveIncidentTarget(t *testing.T) {
	accepted := []struct {
		name   string
		target string
		want   string
	}{
		{"bare numeric", "123", "123"},
		{"bare with space", "a b", "a b"},
		{"bare lookalike path", "incidents.betterstack.com/team/t1/incidents/123", "incidents.betterstack.com/team/t1/incidents/123"},
		{"bare protocol-relative lookalike", "//incidents.betterstack.com/team/t1/incidents/123", "//incidents.betterstack.com/team/t1/incidents/123"},
		{"observed url", "https://incidents.betterstack.com/team/t581731/incidents/1033092002", "1033092002"},
		{"observed url second sample", "https://incidents.betterstack.com/team/team-2/incidents/9", "9"},
		{"uppercase scheme normalizes", "HTTPS://incidents.betterstack.com/team/t1/incidents/123", "123"},
		{"percent-encoded digits decode", "https://incidents.betterstack.com/team/t1/incidents/%31%32%33", "123"},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveIncidentTarget(tc.target)
			if err != nil {
				t.Fatalf("resolveIncidentTarget(%q) error = %v", tc.target, err)
			}
			if got != tc.want {
				t.Errorf("resolveIncidentTarget(%q) = %q, want %q", tc.target, got, tc.want)
			}
		})
	}

	rejected := []struct {
		name   string
		target string
	}{
		{"http scheme", "http://incidents.betterstack.com/team/t1/incidents/123"},
		{"ftp scheme", "ftp://incidents.betterstack.com/team/t1/incidents/123"},
		{"wrong host", "https://uptime.betterstack.com/team/t1/incidents/123"},
		{"near host suffix", "https://incidents.betterstack.com.evil.test/team/t1/incidents/123"},
		{"near host prefix", "https://evil.test/incidents.betterstack.com/team/t1/incidents/123"},
		{"empty host", "https:///team/t1/incidents/123"},
		{"trailing dot host", "https://incidents.betterstack.com./team/t1/incidents/123"},
		{"uppercase host", "https://INCIDENTS.betterstack.com/team/t1/incidents/123"},
		{"explicit port", "https://incidents.betterstack.com:8443/team/t1/incidents/123"},
		{"explicit 443 port", "https://incidents.betterstack.com:443/team/t1/incidents/123"},
		{"userinfo", "https://user:pass@incidents.betterstack.com/team/t1/incidents/123"},
		{"username only", "https://user@incidents.betterstack.com/team/t1/incidents/123"},
		{"query", "https://incidents.betterstack.com/team/t1/incidents/123?x=1"},
		{"empty query marker", "https://incidents.betterstack.com/team/t1/incidents/123?"},
		{"fragment", "https://incidents.betterstack.com/team/t1/incidents/123#frag"},
		{"empty fragment marker", "https://incidents.betterstack.com/team/t1/incidents/123#"},
		{"query and fragment", "https://incidents.betterstack.com/team/t1/incidents/123?x=1#frag"},
		{"trailing slash", "https://incidents.betterstack.com/team/t1/incidents/123/"},
		{"too few segments", "https://incidents.betterstack.com/team/t1/incidents"},
		{"too many segments", "https://incidents.betterstack.com/team/t1/incidents/123/extra"},
		{"empty team", "https://incidents.betterstack.com/team//incidents/123"},
		{"wrong path words", "https://incidents.betterstack.com/org/t1/incidents/123"},
		{"uppercase path words", "https://incidents.betterstack.com/Team/t1/Incidents/123"},
		{"missing terminal id", "https://incidents.betterstack.com/team/t1/incidents/"},
		{"non-numeric terminal", "https://incidents.betterstack.com/team/t1/incidents/abc"},
		{"mixed terminal", "https://incidents.betterstack.com/team/t1/incidents/12a"},
		{"negative terminal", "https://incidents.betterstack.com/team/t1/incidents/-123"},
		{"decimal terminal", "https://incidents.betterstack.com/team/t1/incidents/12.3"},
		{"plus terminal", "https://incidents.betterstack.com/team/t1/incidents/+123"},
		{"unicode digits", "https://incidents.betterstack.com/team/t1/incidents/١٢٣"},
		{"fullwidth digits", "https://incidents.betterstack.com/team/t1/incidents/１２３"},
		{"scheme-ful opaque", "https:team/t1/incidents/123"},
		{"encoded slash in id", "https://incidents.betterstack.com/team/t1/incidents/123%2F456"},
		{"lowercase encoded slash in id", "https://incidents.betterstack.com/team/t1/incidents/123%2f456"},
		{"encoded slash in team", "https://incidents.betterstack.com/team/t%2F1/incidents/123"},
		{"encoded query delimiter", "https://incidents.betterstack.com/team/t1/incidents/123%3Fx"},
		{"encoded fragment delimiter", "https://incidents.betterstack.com/team/t1/incidents/123%23x"},
		{"encoded backslash", "https://incidents.betterstack.com/team/t1/incidents/123%5C456"},
		{"encoded dot dot", "https://incidents.betterstack.com/team/t1/incidents/%2e%2e"},
		{"malformed percent escape", "https://incidents.betterstack.com/team/t1/incidents/123%zz"},
		{"space suffix", "https://incidents.betterstack.com/team/t1/incidents/123 "},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveIncidentTarget(tc.target)
			if err == nil {
				t.Fatalf("resolveIncidentTarget(%q) = %q, nil; want an error", tc.target, got)
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Error("error message is empty, want a readable problem")
			}
		})
	}
}
