package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// @anchor:uptime:config
const (
	defaultAPIBaseURL = "https://incidents.betterstack.com"
	defaultTimeout    = 30 * time.Second
	tokenEnvVar       = "BETTERSTACK_UPTIME_API_TOKEN"
)

// newHTTPClient returns the production HTTP client. Redirect following is
// disabled in monitorsGet so an injected client cannot re-enable it.
func newHTTPClient() *http.Client { return &http.Client{} }

// monitorsGet implements `bs monitors get <id> [--timeout <duration>]`:
// exactly one bearer-authenticated GET, raw success body on stdout, raw
// non-2xx body on stderr, and token-free local failure messages.
// @anchor:uptime:command-handler
func monitorsGet(args []string, stdout, stderr io.Writer, baseURL string, client *http.Client) int {
	// The token is read before argument validation only so locally generated
	// diagnostics can redact it. The missing-token failure still happens only
	// after a valid argument vector (spec R2).
	token := os.Getenv(tokenEnvVar)

	target, timeout, err := parseMonitorsGetArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "bs: %s; run 'bs monitors get --help' for usage.\n", redactToken(err.Error(), token))
		return 1
	}

	if token == "" {
		fmt.Fprintf(stderr, "bs: %s is not set; set it to a Better Stack API token and try again.\n", tokenEnvVar)
		return 1
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/api/v2/monitors/" + url.PathEscape(target)
	return doMonitorsRequest(endpoint, token, timeout, stdout, stderr, client)
}

// monitorsList implements `bs monitors list [flags]`: exactly one
// bearer-authenticated GET, the parsed flags encoded as query parameters,
// raw success body on stdout, raw non-2xx body on stderr, and token-free
// local failure messages. The CLI never follows pagination links.
// @anchor:uptime:command-handler
func monitorsList(args []string, stdout, stderr io.Writer, baseURL string, client *http.Client) int {
	token := os.Getenv(tokenEnvVar)

	parsed, err := parseMonitorsListArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "bs: %s; run 'bs monitors list --help' for usage.\n", redactToken(err.Error(), token))
		return 1
	}

	if token == "" {
		fmt.Fprintf(stderr, "bs: %s is not set; set it to a Better Stack API token and try again.\n", tokenEnvVar)
		return 1
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/api/v2/monitors"
	if query := parsed.query().Encode(); query != "" {
		endpoint += "?" + query
	}
	return doMonitorsRequest(endpoint, token, parsed.timeout, stdout, stderr, client)
}

// doMonitorsRequest performs exactly one bearer-authenticated GET and applies
// the shared 002 response contract used by both monitor commands: raw 2xx body
// to stdout, raw non-2xx body to stderr, redirects not followed, and
// token-free local diagnostics.
// @anchor:uptime:request-executor
func doMonitorsRequest(endpoint, token string, timeout time.Duration, stdout, stderr io.Writer, client *http.Client) int {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		fmt.Fprintf(stderr, "bs: could not build request: %s\n", redactToken(err.Error(), token))
		return 1
	}
	req.Header.Set("Authorization", "Bearer "+token)

	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := c.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "bs: request to the Better Stack API failed: %s\n", redactToken(err.Error(), token))
		return 1
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(stderr, "bs: could not read the API response: %s\n", redactToken(err.Error(), token))
		return 1
	}

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		_, _ = stdout.Write(body)
		return 0
	}
	// The raw upstream body is the authoritative non-2xx output; it is printed
	// byte-for-byte with no redaction or wrapper (spec R4, R7).
	_, _ = stderr.Write(body)
	return 1
}

// redactToken removes the credential value from a locally generated
// diagnostic. It is never applied to a raw upstream response body, which the
// passthrough contract leaves untouched (spec R7).
func redactToken(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "[REDACTED]")
}

func parseMonitorsGetArgs(args []string) (string, time.Duration, error) {
	var target string
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
		case target == "":
			target = arg
		default:
			return "", 0, errors.New("too many arguments")
		}
	}
	if target == "" {
		return "", 0, errors.New("missing <id>")
	}
	return target, timeout, nil
}

// monitorsListArgs holds one parsed `bs monitors list` invocation. A nil
// slice means the flag was absent. A slice containing "" means the flag was
// present with an explicitly empty value. Repeated flags keep every
// occurrence in order so each value reaches upstream unchanged (spec R1.4).
type monitorsListArgs struct {
	pages              []string
	perPages           []string
	urls               []string
	pronounceableNames []string
	teamNames          []string
	timeout            time.Duration
}

// parseMonitorsListArgs parses the flags accepted by `bs monitors list`.
// Every flag consumes the following argument as its value. The timeout uses
// the same duration grammar and invalid-value handling as monitors get; every
// other value passes through unchanged for upstream validation.
func parseMonitorsListArgs(args []string) (monitorsListArgs, error) {
	parsed := monitorsListArgs{timeout: defaultTimeout}
	for i := 0; i < len(args); i++ {
		flag := args[i]
		switch {
		case flag == "--page" || flag == "--per-page" || flag == "--url" || flag == "--pronounceable-name" || flag == "--team-name" || flag == "--timeout":
			if i+1 >= len(args) {
				return monitorsListArgs{}, fmt.Errorf("%s needs a value", flag)
			}
			i++
			value := args[i]
			switch flag {
			case "--page":
				parsed.pages = append(parsed.pages, value)
			case "--per-page":
				parsed.perPages = append(parsed.perPages, value)
			case "--url":
				parsed.urls = append(parsed.urls, value)
			case "--pronounceable-name":
				parsed.pronounceableNames = append(parsed.pronounceableNames, value)
			case "--team-name":
				parsed.teamNames = append(parsed.teamNames, value)
			case "--timeout":
				d, err := time.ParseDuration(value)
				if err != nil || d <= 0 {
					return monitorsListArgs{}, fmt.Errorf("invalid --timeout value %q", value)
				}
				parsed.timeout = d
			}
		case strings.HasPrefix(flag, "-"):
			return monitorsListArgs{}, fmt.Errorf("unknown option %q", flag)
		default:
			return monitorsListArgs{}, fmt.Errorf("unexpected argument %q", flag)
		}
	}
	return parsed, nil
}

// query maps every present flag occurrence onto its API parameter. The
// stdlib url.Values builder performs the encoding (spec R3.2). An absent flag
// adds nothing; an explicit empty value adds the parameter with an empty value.
func (a monitorsListArgs) query() url.Values {
	query := url.Values{}
	for _, value := range a.pages {
		query.Add("page", value)
	}
	for _, value := range a.perPages {
		query.Add("per_page", value)
	}
	for _, value := range a.urls {
		query.Add("url", value)
	}
	for _, value := range a.pronounceableNames {
		query.Add("pronounceable_name", value)
	}
	for _, value := range a.teamNames {
		query.Add("team_name", value)
	}
	return query
}
