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

// parseIncidentsGetArgs parses `bs incidents get <id-or-url> [--timeout d]`.
// It mirrors parseMonitorsGetArgs: one positional target, the same duration
// grammar, and unknown or extra tokens rejected before any request.
func parseIncidentsGetArgs(args []string) (string, time.Duration, error) {
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
		return "", 0, errors.New("missing <id-or-url>")
	}
	return target, timeout, nil
}

// incidentsGet implements `bs incidents get <id-or-url> [--timeout <d>]`:
// argument and target validation run before the credential check, then exactly
// one bearer-authenticated GET uses the shared 002 response contract.
func incidentsGet(args []string, stdout, stderr io.Writer, baseURL string, client *http.Client) int {
	// Read the token before validation only so local diagnostics can redact
	// it; the missing-token failure still occurs after a valid argument vector.
	token := os.Getenv(tokenEnvVar)

	target, timeout, err := parseIncidentsGetArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "bs: %s; run 'bs incidents get --help' for usage.\n", redactToken(err.Error(), token))
		return 1
	}

	candidate, err := resolveIncidentTarget(target)
	if err != nil {
		fmt.Fprintf(stderr, "bs: %s; run 'bs incidents get --help' for usage.\n", redactToken(err.Error(), token))
		return 1
	}

	if token == "" {
		fmt.Fprintf(stderr, "bs: %s is not set; set it to a Better Stack API token and try again.\n", tokenEnvVar)
		return 1
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/api/v3/incidents/" + url.PathEscape(candidate)
	return doMonitorsRequest(endpoint, token, timeout, stdout, stderr, client)
}

// incidentsTimeline implements `bs incidents timeline <id-or-url> [--timeout
// <d>]`: argument and target validation run before the credential check, then
// exactly one bearer-authenticated GET to the incident's timeline endpoint uses
// the shared 002 response contract. The body is opaque: no pagination, no
// completeness promise, and any timeline_truncated item passes through raw.
func incidentsTimeline(args []string, stdout, stderr io.Writer, baseURL string, client *http.Client) int {
	// Read the token before validation only so local diagnostics can redact
	// it; the missing-token failure still occurs after a valid argument vector.
	token := os.Getenv(tokenEnvVar)

	target, timeout, err := parseIncidentsGetArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "bs: %s; run 'bs incidents timeline --help' for usage.\n", redactToken(err.Error(), token))
		return 1
	}

	candidate, err := resolveIncidentTarget(target)
	if err != nil {
		fmt.Fprintf(stderr, "bs: %s; run 'bs incidents timeline --help' for usage.\n", redactToken(err.Error(), token))
		return 1
	}

	if token == "" {
		fmt.Fprintf(stderr, "bs: %s is not set; set it to a Better Stack API token and try again.\n", tokenEnvVar)
		return 1
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/api/v3/incidents/" + url.PathEscape(candidate) + "/timeline"
	return doMonitorsRequest(endpoint, token, timeout, stdout, stderr, client)
}

// resolveIncidentTarget applies the narrow identity rule (spec R2): a target
// the URL parser reads with a scheme must be exactly the observed dashboard
// form, whose terminal ASCII-digit segment is the API incident ID. Everything
// else scheme-less stays an opaque bare ID. No network access happens here.
func resolveIncidentTarget(target string) (string, error) {
	u, err := url.Parse(target)
	if err != nil {
		// A malformed scheme-ful URL is a rejected URL, not an opaque ID.
		// url.Parse returns no URL on invalid escapes, so detect the scheme
		// prefix directly. A scheme-less target stays opaque.
		if scheme, ok := leadingScheme(target); ok {
			return "", fmt.Errorf("invalid incident URL with scheme %q: %v", scheme, err)
		}
		return target, nil
	}
	if u.Scheme == "" {
		return target, nil
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("unsupported incident URL scheme %q; want https", u.Scheme)
	}
	if u.Opaque != "" {
		return "", errors.New("incident URL must use the https://incidents.betterstack.com dashboard form")
	}
	if u.User != nil {
		return "", errors.New("incident URL must not include userinfo")
	}
	if u.Host != "incidents.betterstack.com" {
		return "", fmt.Errorf("incident URL host %q is not incidents.betterstack.com", u.Host)
	}
	if u.RawQuery != "" || u.ForceQuery {
		return "", errors.New("incident URL must not include a query")
	}
	if u.Fragment != "" || strings.Contains(target, "#") {
		return "", errors.New("incident URL must not include a fragment")
	}

	segments := strings.Split(u.Path, "/")
	if len(segments) != 5 || segments[0] != "" || segments[1] != "team" || segments[3] != "incidents" || segments[2] == "" {
		return "", errors.New("incident URL path must be /team/<team-id>/incidents/<incident-id>")
	}
	id := segments[4]
	if id == "" || !asciiDigits(id) {
		return "", errors.New("incident URL <incident-id> must be one or more ASCII digits")
	}
	return id, nil
}

func asciiDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

// leadingScheme reports whether target starts with an RFC 3986 scheme prefix.
// It is used only when url.Parse has already failed, so the scheme cannot be
// read from a parsed URL.
func leadingScheme(target string) (string, bool) {
	for i := 0; i < len(target); i++ {
		c := target[i]
		switch {
		case c == ':':
			return target[:i], i > 0
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
			// Letters are valid in every scheme position.
		case i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
			// Digits and the three punctuation characters are valid after
			// the first letter.
		default:
			return "", false
		}
	}
	return "", false
}
