package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var logsTablePattern = regexp.MustCompile(`^t[0-9]+_[a-z0-9_]+$`)

type logsSearchArgs struct {
	table   string
	cold    bool
	query   *string
	order   string
	limit   int
	since   time.Duration
	timeout time.Duration
	values  map[string]string
}

func parseLogsSearchArgs(args []string) (logsSearchArgs, error) {
	parsed := logsSearchArgs{values: make(map[string]string), order: "desc", limit: 100, since: time.Hour, timeout: defaultTimeout}
	for i := 0; i < len(args); i++ {
		flag := args[i]
		switch flag {
		case "--cold":
			parsed.cold = true
		case "--table", "--since", "--from", "--to", "--query", "--order", "--limit", "--timeout":
			if _, exists := parsed.values[flag]; exists {
				return logsSearchArgs{}, fmt.Errorf("duplicate %s", flag)
			}
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return logsSearchArgs{}, fmt.Errorf("%s needs a value", flag)
			}
			i++
			parsed.values[flag] = args[i]
		default:
			if strings.HasPrefix(flag, "-") {
				return logsSearchArgs{}, fmt.Errorf("unknown option %q", flag)
			}
			return logsSearchArgs{}, fmt.Errorf("unexpected argument %q", flag)
		}
	}
	parsed.table = parsed.values["--table"]
	if !logsTablePattern.MatchString(parsed.table) {
		return logsSearchArgs{}, fmt.Errorf("invalid or missing --table value %q", parsed.table)
	}
	if value, present := parsed.values["--query"]; present {
		parsed.query = &value
	}
	if value, present := parsed.values["--order"]; present {
		if value != "asc" && value != "desc" {
			return logsSearchArgs{}, fmt.Errorf("invalid --order value %q", value)
		}
		parsed.order = value
	}
	if value, present := parsed.values["--limit"]; present {
		if !validSourcePagination(value, false) {
			return logsSearchArgs{}, fmt.Errorf("invalid --limit value %q", value)
		}
		limit, err := strconv.Atoi(strings.TrimLeft(value, "0"))
		if err != nil || limit > 1000 {
			return logsSearchArgs{}, fmt.Errorf("invalid --limit value %q", value)
		}
		parsed.limit = limit
	}
	for _, flag := range []string{"--since", "--timeout"} {
		if value, present := parsed.values[flag]; present {
			duration, err := time.ParseDuration(value)
			if err != nil || duration <= 0 {
				return logsSearchArgs{}, fmt.Errorf("invalid %s value %q", flag, value)
			}
			if flag == "--since" {
				parsed.since = duration
			} else {
				parsed.timeout = duration
			}
		}
	}
	return parsed, nil
}
