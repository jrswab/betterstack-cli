package main

import (
	"strings"
	"testing"
	"time"
)

// TestParseLogsSearchArgsTable verifies the required full table identifier grammar.
func TestParseLogsSearchArgsTable(t *testing.T) {
	for _, table := range []string{"t1_a", "t123_fake_logs_01", "t0__", "t001_0"} {
		parsed, err := parseLogsSearchArgs([]string{"--table", table})
		if err != nil || parsed.table != table {
			t.Fatalf("table = %q, error = %v", parsed.table, err)
		}
	}
	for _, table := range []string{"", "fake", "t1_", "t_a", "t1_A", "T1_a", "t1_a.b", "t1_a-b", "t1_a;DROP", "t1_a b", " t1_a", "t1_a\n", "t١_a", "t1_é"} {
		t.Run(table, func(t *testing.T) {
			_, err := parseLogsSearchArgs([]string{"--table", table})
			if err == nil || !strings.Contains(err.Error(), "--table") {
				t.Fatalf("error = %v, want --table diagnostic", err)
			}
		})
	}
	if _, err := parseLogsSearchArgs(nil); err == nil || !strings.Contains(err.Error(), "--table") {
		t.Fatalf("missing table error = %v", err)
	}
}

// TestParseLogsSearchArgsSyntax verifies space-separated values and rejects extra arguments.
func TestParseLogsSearchArgsSyntax(t *testing.T) {
	for _, flag := range []string{"--table", "--since", "--from", "--to", "--query", "--order", "--limit", "--timeout"} {
		for _, tail := range [][]string{{flag}, {flag + "=x"}, {flag, "--cold"}} {
			args := tail
			if flag != "--table" {
				args = append([]string{"--table", "t1_fake"}, tail...)
			}
			_, err := parseLogsSearchArgs(args)
			if err == nil || !strings.Contains(err.Error(), flag) {
				t.Fatalf("args = %q, error = %v", args, err)
			}
		}
	}
	for _, tail := range [][]string{{"--unknown"}, {"positional"}, {""}, {"--cold=true"}, {"--cold", "true"}} {
		if _, err := parseLogsSearchArgs(append([]string{"--table", "t1_fake"}, tail...)); err == nil {
			t.Fatalf("accepted args %q", tail)
		}
	}
}

// TestParseLogsSearchArgsDuplicates verifies every single-value flag rejects repetition first.
func TestParseLogsSearchArgsDuplicates(t *testing.T) {
	values := map[string]string{
		"--table": "t1_fake", "--since": "1h", "--from": "2026-01-01T00:00:00Z", "--to": "2026-01-02T00:00:00Z",
		"--query": "fake", "--order": "asc", "--limit": "1", "--timeout": "1s",
	}
	for flag, value := range values {
		for _, repeated := range [][]string{{flag, value}, {flag, "different"}, {flag}, {flag, "--cold"}} {
			t.Run(flag+"/"+strings.Join(repeated, " "), func(t *testing.T) {
				args := []string{flag, value}
				if flag != "--table" {
					args = append([]string{"--table", "t1_fake"}, args...)
				}
				_, err := parseLogsSearchArgs(append(args, repeated...))
				if err == nil || !strings.Contains(err.Error(), flag) || !strings.Contains(err.Error(), "duplicate") {
					t.Fatalf("error = %v, want duplicate %s", err, flag)
				}
			})
		}
	}
}

// TestParseLogsSearchArgsColdAndQuery verifies an idempotent switch and query presence.
func TestParseLogsSearchArgsColdAndQuery(t *testing.T) {
	parsed, err := parseLogsSearchArgs([]string{"--cold", "--table", "t1_fake", "--cold"})
	if err != nil || !parsed.cold || parsed.query != nil {
		t.Fatalf("cold = %t, query = %v, error = %v", parsed.cold, parsed.query, err)
	}
	for _, query := range []string{"", "fake text"} {
		parsed, err := parseLogsSearchArgs([]string{"--query", query, "--table", "t1_fake"})
		if err != nil || parsed.cold || parsed.query == nil || *parsed.query != query {
			t.Fatalf("query = %v, cold = %t, error = %v", parsed.query, parsed.cold, err)
		}
	}
}

// TestParseLogsSearchArgsDefaults verifies the approved search defaults.
func TestParseLogsSearchArgsDefaults(t *testing.T) {
	parsed, err := parseLogsSearchArgs([]string{"--table", "t1_fake"})
	if err != nil || parsed.order != "desc" || parsed.limit != 100 || parsed.since != time.Hour || parsed.timeout != 30*time.Second {
		t.Fatalf("parsed = %+v, error = %v", parsed, err)
	}
}

// TestParseLogsSearchArgsValues verifies allowed order, limit, and duration values.
func TestParseLogsSearchArgsValues(t *testing.T) {
	for _, order := range []string{"asc", "desc"} {
		parsed, err := parseLogsSearchArgs([]string{"--table", "t1_fake", "--order", order})
		if err != nil || parsed.order != order {
			t.Fatalf("order = %q, error = %v", parsed.order, err)
		}
	}
	for _, tc := range []struct {
		value string
		want  int
	}{{"1", 1}, {"1000", 1000}, {"00100", 100}} {
		parsed, err := parseLogsSearchArgs([]string{"--table", "t1_fake", "--limit", tc.value})
		if err != nil || parsed.limit != tc.want {
			t.Fatalf("limit = %d, error = %v", parsed.limit, err)
		}
	}
	for _, flag := range []string{"--since", "--timeout"} {
		for _, tc := range []struct {
			value string
			want  time.Duration
		}{{"1ns", time.Nanosecond}, {"30m", 30 * time.Minute}, {"1h", time.Hour}, {"2h45m", 165 * time.Minute}, {"168h", 168 * time.Hour}} {
			parsed, err := parseLogsSearchArgs([]string{"--table", "t1_fake", flag, tc.value})
			got := parsed.since
			if flag == "--timeout" {
				got = parsed.timeout
			}
			if err != nil || got != tc.want {
				t.Fatalf("%s = %s, error = %v", flag, got, err)
			}
		}
	}
}

// TestParseLogsSearchArgsRejectsValues verifies strict enums, decimal limits, and positive durations.
func TestParseLogsSearchArgsRejectsValues(t *testing.T) {
	values := map[string][]string{
		"--order":   {"", "ASC", "Desc", "newest", " asc"},
		"--limit":   {"", "0", "000", "-1", "+1", "1001", "1.5", "1e2", "one", " 1", "1 ", "١", "99999999999999999999999999999999"},
		"--since":   {"", "7d", "bad", "0", "0s", "-1h", "1", "1e3s", "999999999999999999h"},
		"--timeout": {"", "7d", "bad", "0s", "-1s", "999999999999999999h"},
	}
	for flag, inputs := range values {
		for _, value := range inputs {
			t.Run(flag+"/"+value, func(t *testing.T) {
				_, err := parseLogsSearchArgs([]string{"--table", "t1_fake", flag, value})
				if err == nil || !strings.Contains(err.Error(), flag) {
					t.Fatalf("error = %v, want %s diagnostic", err, flag)
				}
			})
		}
	}
}
