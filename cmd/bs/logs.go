package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
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

func buildLogsSearchQuery(args []string, now time.Time) (string, url.Values, error) {
	parsed, err := parseLogsSearchArgs(args)
	if err != nil {
		return "", nil, err
	}
	from, to := now.Add(-parsed.since), now
	fromValue, hasFrom := parsed.values["--from"]
	toValue, hasTo := parsed.values["--to"]
	if _, hasSince := parsed.values["--since"]; hasSince && (hasFrom || hasTo) {
		return "", nil, fmt.Errorf("--since cannot be combined with --from or --to")
	}
	if hasTo && !hasFrom {
		return "", nil, fmt.Errorf("--to requires --from")
	}
	if hasFrom {
		from, err = time.Parse(time.RFC3339Nano, fromValue)
		if err != nil {
			return "", nil, fmt.Errorf("invalid --from value %q", fromValue)
		}
	}
	if hasTo {
		to, err = time.Parse(time.RFC3339Nano, toValue)
		if err != nil {
			return "", nil, fmt.Errorf("invalid --to value %q", toValue)
		}
	}
	if !from.Before(to) {
		return "", nil, fmt.Errorf("--from must be before --to")
	}
	params := url.Values{
		"param_from": {strconv.FormatInt(from.UnixMilli(), 10)},
		"param_to":   {strconv.FormatInt(to.UnixMilli(), 10)},
	}
	predicate := "dt >= fromUnixTimestamp64Milli({from:Int64}, 'UTC') AND dt < fromUnixTimestamp64Milli({to:Int64}, 'UTC')"
	if parsed.query != nil {
		predicate += " AND positionCaseInsensitiveUTF8(raw, {query:String}) > 0"
		params.Set("param_query", *parsed.query)
	}
	seconds := parsed.timeout / time.Second
	if parsed.timeout%time.Second != 0 {
		seconds++
	}
	sql := fmt.Sprintf("SELECT dt, raw FROM remote(%s_logs) WHERE %s ORDER BY dt %s LIMIT %d SETTINGS max_execution_time = %d FORMAT JSONEachRow", parsed.table, predicate, strings.ToUpper(parsed.order), parsed.limit, seconds)
	return sql, params, nil
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
			if i+1 >= len(args) || (flag != "--query" && strings.HasPrefix(args[i+1], "--")) {
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
		if strings.ContainsFunc(value, unicode.IsControl) {
			return logsSearchArgs{}, fmt.Errorf("invalid --query value: control characters are not allowed")
		}
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
