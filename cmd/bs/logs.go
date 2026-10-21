package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// @anchor:telemetry:config sql
const (
	telemetrySQLURLEnvVar      = "BETTERSTACK_TELEMETRY_SQL_URL"
	telemetrySQLUsernameEnvVar = "BETTERSTACK_TELEMETRY_SQL_USERNAME"
	telemetrySQLPasswordEnvVar = "BETTERSTACK_TELEMETRY_SQL_PASSWORD"
)

type logsSQLRequest struct {
	sql     string
	params  url.Values
	timeout time.Duration
}

// doLogsSQLRequest sends SQL with separate Basic credentials and safe local diagnostics.
// @anchor:telemetry:request-executor sql
func doLogsSQLRequest(sql string, params url.Values, timeout time.Duration, stdout, stderr io.Writer, telemetrySQLURL string, client *http.Client) int {
	values := make([]string, 0, 3)
	for _, name := range []string{telemetrySQLURLEnvVar, telemetrySQLUsernameEnvVar, telemetrySQLPasswordEnvVar} {
		value := os.Getenv(name)
		if value == "" {
			fmt.Fprintf(stderr, "bs: %s is not set\n", name)
			return 1
		}
		values = append(values, value)
	}
	endpoint, err := parseTelemetrySQLURL(values[0])
	if err != nil {
		fmt.Fprintf(stderr, "bs: %s\n", err)
		return 1
	}
	if telemetrySQLURL != "" {
		endpoint, err = url.Parse(telemetrySQLURL)
		if err != nil {
			fmt.Fprintln(stderr, "bs: could not build SQL request")
			return 1
		}
	}
	endpoint.RawQuery = params.Encode()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(sql))
	if err != nil {
		fmt.Fprintln(stderr, "bs: could not build SQL request")
		return 1
	}
	req.SetBasicAuth(values[1], values[2])
	req.Header.Set("Content-Type", "plain/text")
	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := c.Do(req)
	if err != nil {
		fmt.Fprintln(stderr, "bs: SQL request failed")
		return 1
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(stderr, "bs: could not read SQL response")
		return 1
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		_, _ = stdout.Write(body)
		return 0
	}
	_, _ = io.WriteString(stderr, redactSecrets(string(body), os.Getenv(telemetryTokenEnvVar), values[1], values[2]))
	return 1
}

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

func parseTelemetrySQLURL(value string) (*url.URL, error) {
	invalid := fmt.Errorf("invalid BETTERSTACK_TELEMETRY_SQL_URL")
	absolute := strings.HasPrefix(value, "https://")
	if !absolute {
		if strings.ContainsAny(value, "/?#@") {
			return nil, invalid
		}
		value = "https://" + value
	}
	endpoint, err := url.Parse(value)
	if err != nil || endpoint.Scheme != "https" || endpoint.Opaque != "" || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery || strings.Contains(value, "#") || (endpoint.EscapedPath() != "" && endpoint.EscapedPath() != "/") {
		return nil, invalid
	}
	host := endpoint.Hostname()
	if strings.HasPrefix(endpoint.Host, "[") {
		ip, err := netip.ParseAddr(host)
		if err != nil || !ip.Is6() || ip.Zone() != "" {
			return nil, invalid
		}
	} else {
		if len(host) == 0 || len(host) > 253 {
			return nil, invalid
		}
		for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return nil, invalid
			}
			for _, c := range label {
				if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
					return nil, invalid
				}
			}
		}
	}
	port := endpoint.Port()
	if strings.HasSuffix(endpoint.Host, ":") {
		return nil, invalid
	}
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 || (!absolute && port != "443") {
			return nil, invalid
		}
	}
	if !absolute && endpoint.Path != "" {
		return nil, invalid
	}
	return endpoint, nil
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
	source := fmt.Sprintf("remote(%s_logs)", parsed.table)
	if parsed.cold {
		source = fmt.Sprintf("(SELECT dt, raw FROM %s UNION ALL SELECT dt, raw FROM s3Cluster(primary, %s_s3) WHERE _row_type = 1)", source, parsed.table)
	}
	sql := fmt.Sprintf("SELECT dt, raw FROM %s WHERE %s ORDER BY dt %s LIMIT %d SETTINGS max_execution_time = %d FORMAT JSONEachRow", source, predicate, strings.ToUpper(parsed.order), parsed.limit, seconds)
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
