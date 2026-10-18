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
