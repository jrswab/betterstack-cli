package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// @anchor:telemetry:config
const telemetryTokenEnvVar = "BETTERSTACK_TELEMETRY_API_TOKEN"

// sourcesList sends one Telemetry source-list GET through the shared request path.
// @anchor:telemetry:command-handler
func sourcesList(args []string, stdout, stderr io.Writer, baseURL string, client *http.Client) int {
	token := os.Getenv(telemetryTokenEnvVar)
	parsed, err := parseSourcesListArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "bs: %s; run 'bs sources list --help' for usage.\n", redactToken(err.Error(), token))
		return 1
	}
	if token == "" {
		fmt.Fprintf(stderr, "bs: %s is not set; set it to a Better Stack API token and try again.\n", telemetryTokenEnvVar)
		return 1
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/v2/sources"
	if query := parsed.query().Encode(); query != "" {
		endpoint += "?" + query
	}
	return doMonitorsRequest(endpoint, token, parsed.timeout, stdout, stderr, client)
}

// sourcesGet sends one Telemetry source GET with an opaque escaped target.
// @anchor:telemetry:command-handler
func sourcesGet(args []string, stdout, stderr io.Writer, baseURL string, client *http.Client) int {
	token := os.Getenv(telemetryTokenEnvVar)
	target, timeout, err := parseSourcesGetArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "bs: %s; run 'bs sources get --help' for usage.\n", redactToken(err.Error(), token))
		return 1
	}
	if token == "" {
		fmt.Fprintf(stderr, "bs: %s is not set; set it to a Better Stack API token and try again.\n", telemetryTokenEnvVar)
		return 1
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/v2/sources/" + url.PathEscape(target)
	return doMonitorsRequest(endpoint, token, timeout, stdout, stderr, client)
}

func parseSourcesGetArgs(args []string) (string, time.Duration, error) {
	var target string
	hasTarget := false
	timeout := defaultTimeout
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--timeout":
			if i+1 >= len(args) {
				return "", 0, errors.New("--timeout needs a duration value")
			}
			i++
			d, err := time.ParseDuration(args[i])
			if err != nil || d <= 0 {
				return "", 0, fmt.Errorf("invalid --timeout value %q", args[i])
			}
			timeout = d
		case strings.HasPrefix(arg, "-"):
			return "", 0, fmt.Errorf("unknown option %q", arg)
		case hasTarget:
			return "", 0, errors.New("too many arguments")
		default:
			target = arg
			hasTarget = true
		}
	}
	if target == "" {
		return "", 0, errors.New("missing <id>")
	}
	return target, timeout, nil
}

type sourcesListArgs struct {
	pages    []string
	perPages []string
	timeout  time.Duration
}

func parseSourcesListArgs(args []string) (sourcesListArgs, error) {
	parsed := sourcesListArgs{timeout: defaultTimeout}
	for i := 0; i < len(args); i++ {
		flag := args[i]
		switch flag {
		case "--page", "--per-page", "--timeout":
			if i+1 >= len(args) {
				return sourcesListArgs{}, fmt.Errorf("%s needs a value", flag)
			}
			i++
			value := args[i]
			if flag == "--timeout" {
				d, err := time.ParseDuration(value)
				if err != nil || d <= 0 {
					return sourcesListArgs{}, fmt.Errorf("invalid --timeout value %q", value)
				}
				parsed.timeout = d
				continue
			}
			if !validSourcePagination(value, flag == "--per-page") {
				return sourcesListArgs{}, fmt.Errorf("invalid %s value %q", flag, value)
			}
			if flag == "--page" {
				parsed.pages = append(parsed.pages, value)
			} else {
				parsed.perPages = append(parsed.perPages, value)
			}
		default:
			if strings.HasPrefix(flag, "-") {
				return sourcesListArgs{}, fmt.Errorf("unknown option %q", flag)
			}
			return sourcesListArgs{}, fmt.Errorf("unexpected argument %q", flag)
		}
	}
	return parsed, nil
}

func validSourcePagination(value string, perPage bool) bool {
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	integer := strings.TrimLeft(value, "0")
	if integer == "" {
		return false
	}
	return !perPage || len(integer) < 2 || (len(integer) == 2 && integer <= "50")
}

func (a sourcesListArgs) query() url.Values {
	query := url.Values{}
	for _, value := range a.pages {
		query.Add("page", value)
	}
	for _, value := range a.perPages {
		query.Add("per_page", value)
	}
	return query
}
