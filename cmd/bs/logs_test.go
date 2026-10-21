package main

import (
	"reflect"
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
			if flag == "--query" && len(tail) == 2 {
				continue
			}
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

// TestBuildLogsSearchQueryHot verifies the complete default and explicit hot query.
func TestBuildLogsSearchQueryHot(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"default", nil, "SELECT dt, raw FROM remote(t1_fake_logs) WHERE dt >= fromUnixTimestamp64Milli({from:Int64}, 'UTC') AND dt < fromUnixTimestamp64Milli({to:Int64}, 'UTC') ORDER BY dt DESC LIMIT 100 SETTINGS max_execution_time = 30 FORMAT JSONEachRow"},
		{"explicit", []string{"--order", "asc", "--limit", "1000", "--timeout", "2s"}, "SELECT dt, raw FROM remote(t1_fake_logs) WHERE dt >= fromUnixTimestamp64Milli({from:Int64}, 'UTC') AND dt < fromUnixTimestamp64Milli({to:Int64}, 'UTC') ORDER BY dt ASC LIMIT 1000 SETTINGS max_execution_time = 2 FORMAT JSONEachRow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sql, params, err := buildLogsSearchQuery(append([]string{"--table", "t1_fake"}, tc.args...), time.Unix(3600, 0))
			if err != nil || sql != tc.want {
				t.Fatalf("SQL = %q, error = %v", sql, err)
			}
			if !reflect.DeepEqual(map[string][]string(params), map[string][]string{"param_from": {"0"}, "param_to": {"3600000"}}) {
				t.Fatalf("params = %v", params)
			}
		})
	}
}

// TestBuildLogsSearchQueryText verifies that search text stays in a typed parameter.
func TestBuildLogsSearchQueryText(t *testing.T) {
	for _, text := range []string{"", "MiXeD fake", "fake 'quotes' \"double\"", `fake\path`, "fake %_.*", "fake'); DROP TABLE fake; --", "--fake", "faké 世界"} {
		t.Run(text, func(t *testing.T) {
			sql, params, err := buildLogsSearchQuery([]string{"--table", "t1_fake", "--query", text}, time.Unix(3600, 0))
			if err != nil || !strings.Contains(sql, "AND positionCaseInsensitiveUTF8(raw, {query:String}) > 0") {
				t.Fatalf("SQL = %q, error = %v", sql, err)
			}
			value, present := params["param_query"]
			if !present || !reflect.DeepEqual(value, []string{text}) || len(params) != 3 {
				t.Fatalf("params = %v", params)
			}
			if text != "" && strings.Contains(sql, text) {
				t.Fatalf("SQL contains query text: %q", sql)
			}
		})
	}
}

// TestBuildLogsSearchQueryRejectsControls verifies control text never produces a query.
func TestBuildLogsSearchQueryRejectsControls(t *testing.T) {
	for _, text := range []string{"fake\x00text", "fake\ntext", "fake\rtext", "fake\ttext", "fake\x1btext", "fake\x7ftext", "fake\u0085text"} {
		sql, params, err := buildLogsSearchQuery([]string{"--table", "t1_fake", "--query", text}, time.Unix(3600, 0))
		if err == nil || !strings.Contains(err.Error(), "--query") || sql != "" || params != nil {
			t.Fatalf("SQL = %q, params = %v, error = %v", sql, params, err)
		}
	}
}

// TestBuildLogsSearchQueryTimeout verifies integer ceiling without duration overflow.
func TestBuildLogsSearchQueryTimeout(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  string
	}{{"1ns", "1"}, {"250ms", "1"}, {"1s", "1"}, {"5s", "5"}, {"5.01s", "6"}, {"2562047h47m16.854775807s", "9223372037"}} {
		sql, _, err := buildLogsSearchQuery([]string{"--table", "t1_fake", "--timeout", tc.value}, time.Unix(3600, 0))
		if err != nil || !strings.Contains(sql, "SETTINGS max_execution_time = "+tc.want+" FORMAT JSONEachRow") {
			t.Fatalf("timeout %s: SQL = %q, error = %v", tc.value, sql, err)
		}
	}
}

// TestBuildLogsSearchQueryRejectsArguments verifies parsing failure returns no SQL or parameters.
func TestBuildLogsSearchQueryRejectsArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"--table", "t1_fake); DROP"}, {"--table", "t1_fake", "--limit", "0"}, {"--table", "t1_fake", "--order", "ASC"}, {"--table", "t1_fake", "--timeout", "0s"}, {"--table", "t1_fake", "--since", "7d"}} {
		sql, params, err := buildLogsSearchQuery(args, time.Unix(3600, 0))
		if err == nil || sql != "" || params != nil {
			t.Fatalf("args = %q, SQL = %q, params = %v, error = %v", args, sql, params, err)
		}
	}
}

// TestBuildLogsSearchQueryTimeBounds verifies fixed-clock windows and RFC3339Nano conversion.
func TestBuildLogsSearchQueryTimeBounds(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		from string
		to   string
	}{
		{"default", nil, "3600123", "7200123"},
		{"half hour", []string{"--since", "30m"}, "5400123", "7200123"},
		{"one hour", []string{"--since", "1h"}, "3600123", "7200123"},
		{"compound", []string{"--since", "2h45m"}, "-2699877", "7200123"},
		{"submillisecond since", []string{"--since", "1ns"}, "7200123", "7200123"},
		{"both", []string{"--from", "1970-01-01T00:00:01Z", "--to", "1970-01-01T00:00:02Z"}, "1000", "2000"},
		{"from only", []string{"--from", "1970-01-01T00:00:01Z"}, "1000", "7200123"},
		{"offset", []string{"--from", "1970-01-01T02:00:00+01:00", "--to", "1970-01-01T03:00:00+01:00"}, "3600000", "7200000"},
		{"nanoseconds", []string{"--from", "1970-01-01T00:00:01.123456789Z", "--to", "1970-01-01T00:00:02.987654321Z"}, "1123", "2987"},
		{"same millisecond", []string{"--from", "1970-01-01T00:00:01.0001Z", "--to", "1970-01-01T00:00:01.0009Z"}, "1000", "1000"},
		{"pre epoch", []string{"--from", "1969-12-31T23:59:59.9991Z", "--to", "1970-01-01T00:00:00.0001Z"}, "-1", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sql, params, err := buildLogsSearchQuery(append([]string{"--table", "t1_fake"}, tc.args...), time.Unix(7200, 123456789))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(map[string][]string(params), map[string][]string{"param_from": {tc.from}, "param_to": {tc.to}}) {
				t.Fatalf("params = %v", params)
			}
			if !strings.Contains(sql, "dt >= fromUnixTimestamp64Milli({from:Int64}, 'UTC') AND dt < fromUnixTimestamp64Milli({to:Int64}, 'UTC')") {
				t.Fatalf("SQL = %q", sql)
			}
		})
	}
}

// TestBuildLogsSearchQueryRejectsTimeBounds verifies invalid and mutually exclusive time flags.
func TestBuildLogsSearchQueryRejectsTimeBounds(t *testing.T) {
	for _, tc := range []struct {
		args []string
		flag string
	}{
		{[]string{"--to", "1970-01-01T00:00:02Z"}, "--to"},
		{[]string{"--from", ""}, "--from"},
		{[]string{"--from", "1970-01-01"}, "--from"},
		{[]string{"--from", "fake'); DROP TABLE fake; --"}, "--from"},
		{[]string{"--from", "1970-13-01T00:00:00Z"}, "--from"},
		{[]string{"--from", "1970-01-01T00:00:00Z", "--to", ""}, "--to"},
		{[]string{"--from", "1970-01-01T00:00:00Z", "--to", "bad"}, "--to"},
		{[]string{"--from", "1970-01-01T00:00:02Z", "--to", "1970-01-01T00:00:02Z"}, "--from"},
		{[]string{"--from", "1970-01-01T00:00:03Z", "--to", "1970-01-01T00:00:02Z"}, "--from"},
		{[]string{"--from", "1970-01-01T02:00:00.123456789Z"}, "--from"},
		{[]string{"--from", "1970-01-01T03:00:00Z"}, "--from"},
		{[]string{"--since", "1h", "--from", "1970-01-01T00:00:01Z"}, "--since"},
		{[]string{"--since", "1h", "--to", "1970-01-01T00:00:02Z"}, "--since"},
		{[]string{"--from", "1970-01-01T00:00:01Z", "--to", "1970-01-01T00:00:02Z", "--since", "1h"}, "--since"},
	} {
		sql, params, err := buildLogsSearchQuery(append([]string{"--table", "t1_fake"}, tc.args...), time.Unix(7200, 123456789))
		if err == nil || !strings.Contains(err.Error(), tc.flag) || sql != "" || params != nil {
			t.Fatalf("args = %q, SQL = %q, params = %v, error = %v", tc.args, sql, params, err)
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
