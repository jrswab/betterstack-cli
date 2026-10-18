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

// monitorsGet implements `bs monitors get <id>`. The HTTP request path lands
// here; credential handling, output passthrough, and timeout parsing follow in
// later checklist tasks.
func monitorsGet(args []string, stdout, stderr io.Writer, baseURL string, client *http.Client) int {
	target, err := parseMonitorsGetTarget(args)
	if err != nil {
		fmt.Fprintf(stderr, "bs: %s; run 'bs monitors get --help' for usage.\n", err)
		return 1
	}

	token := os.Getenv(tokenEnvVar)
	if token == "" {
		fmt.Fprintf(stderr, "bs: %s is not set; set it to a Better Stack API token and try again.\n", tokenEnvVar)
		return 1
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/api/v2/monitors/" + url.PathEscape(target)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		fmt.Fprintf(stderr, "bs: could not build request: %s\n", err)
		return 1
	}
	req.Header.Set("Authorization", "Bearer "+token)

	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := c.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "bs: request to the Better Stack API failed: %s\n", err)
		return 1
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(stderr, "bs: could not read the API response: %s\n", err)
		return 1
	}

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		_, _ = stdout.Write(body)
		return 0
	}
	return 0
}

func parseMonitorsGetTarget(args []string) (string, error) {
	var target string
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "-"):
			return "", fmt.Errorf("unknown option %q", arg)
		case target == "":
			target = arg
		default:
			return "", errors.New("too many arguments")
		}
	}
	if target == "" {
		return "", errors.New("missing <id>")
	}
	return target, nil
}
