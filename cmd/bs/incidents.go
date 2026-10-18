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

// incidentsListArgs holds one parsed `bs incidents list` invocation. A nil
// slice means the flag was absent. A slice containing "" means the flag was
// present with an explicitly empty value. Repeated flags keep every
// occurrence in order so each value reaches upstream unchanged (spec R1.6).
type incidentsListArgs struct {
	pages         []string
	perPages      []string
	froms         []string
	tos           []string
	monitorIDs    []string
	heartbeatIDs  []string
	teamNames     []string
	resolveds     []string
	acknowledgeds []string
	metadatas     []metadataPair
	timeout       time.Duration
}

// metadataPair is one accepted `--metadata key=value` occurrence. The key is
// opaque and passes through unchanged (spec R1.4); the stdlib query builder
// percent-encodes it with the rest of the nested parameter name (spec R3.3).
type metadataPair struct {
	key   string
	value string
}

// parseIncidentsListArgs parses the flags accepted by `bs incidents list`.
// Every non-boolean flag consumes the following argument as its value. The
// boolean flags accept both the space form (`--resolved true`) and the
// documented equals form (`--resolved=true`); only the exact lowercase
// literals `true` and `false` are valid (spec R1.3). Duplicate-key detection
// is deferred to duplicateMetadataKey so the parser stays a per-occurrence
// recorder (spec R1.5).
func parseIncidentsListArgs(args []string) (incidentsListArgs, error) {
	parsed := incidentsListArgs{timeout: defaultTimeout}
	for i := 0; i < len(args); i++ {
		flag := args[i]
		switch {
		case flag == "--page" || flag == "--per-page" || flag == "--from" || flag == "--to" || flag == "--monitor-id" || flag == "--heartbeat-id" || flag == "--team-name" || flag == "--timeout":
			if i+1 >= len(args) {
				return incidentsListArgs{}, fmt.Errorf("%s needs a value", flag)
			}
			i++
			value := args[i]
			switch flag {
			case "--page":
				parsed.pages = append(parsed.pages, value)
			case "--per-page":
				parsed.perPages = append(parsed.perPages, value)
			case "--from":
				parsed.froms = append(parsed.froms, value)
			case "--to":
				parsed.tos = append(parsed.tos, value)
			case "--monitor-id":
				parsed.monitorIDs = append(parsed.monitorIDs, value)
			case "--heartbeat-id":
				parsed.heartbeatIDs = append(parsed.heartbeatIDs, value)
			case "--team-name":
				parsed.teamNames = append(parsed.teamNames, value)
			case "--timeout":
				d, err := time.ParseDuration(value)
				if err != nil || d <= 0 {
					return incidentsListArgs{}, fmt.Errorf("invalid --timeout value %q", value)
				}
				parsed.timeout = d
			}
		case flag == "--resolved" || flag == "--acknowledged":
			if i+1 >= len(args) {
				return incidentsListArgs{}, fmt.Errorf("%s needs a value", flag)
			}
			i++
			value := args[i]
			if !validBoolLiteral(value) {
				return incidentsListArgs{}, fmt.Errorf("invalid %s value %q; want true or false", flag, value)
			}
			if flag == "--resolved" {
				parsed.resolveds = append(parsed.resolveds, value)
			} else {
				parsed.acknowledgeds = append(parsed.acknowledgeds, value)
			}
		case strings.HasPrefix(flag, "--resolved="):
			value := strings.TrimPrefix(flag, "--resolved=")
			if !validBoolLiteral(value) {
				return incidentsListArgs{}, fmt.Errorf("invalid --resolved value %q; want true or false", value)
			}
			parsed.resolveds = append(parsed.resolveds, value)
		case strings.HasPrefix(flag, "--acknowledged="):
			value := strings.TrimPrefix(flag, "--acknowledged=")
			if !validBoolLiteral(value) {
				return incidentsListArgs{}, fmt.Errorf("invalid --acknowledged value %q; want true or false", value)
			}
			parsed.acknowledgeds = append(parsed.acknowledgeds, value)
		case flag == "--metadata":
			if i+1 >= len(args) {
				return incidentsListArgs{}, errors.New("--metadata needs a value")
			}
			i++
			pair, err := parseMetadataPair(args[i])
			if err != nil {
				return incidentsListArgs{}, err
			}
			parsed.metadatas = append(parsed.metadatas, pair)
		case strings.HasPrefix(flag, "-"):
			return incidentsListArgs{}, fmt.Errorf("unknown option %q", flag)
		default:
			return incidentsListArgs{}, fmt.Errorf("unexpected argument %q", flag)
		}
	}
	return parsed, nil
}

func validBoolLiteral(value string) bool {
	return value == "true" || value == "false"
}

// parseMetadataPair splits a `--metadata` value at its first `=`. The key must
// be nonempty; the value may be empty and may itself contain `=` (spec R1.4).
func parseMetadataPair(raw string) (metadataPair, error) {
	key, value, ok := strings.Cut(raw, "=")
	if !ok {
		return metadataPair{}, fmt.Errorf("invalid --metadata value %q; want key=value", raw)
	}
	if key == "" {
		return metadataPair{}, fmt.Errorf("invalid --metadata value %q; the key is empty", raw)
	}
	return metadataPair{key: key, value: value}, nil
}

// duplicateMetadataKey returns the first exact, case-sensitive repeated key.
func duplicateMetadataKey(pairs []metadataPair) (string, bool) {
	seen := make(map[string]struct{}, len(pairs))
	for _, pair := range pairs {
		if _, ok := seen[pair.key]; ok {
			return pair.key, true
		}
		seen[pair.key] = struct{}{}
	}
	return "", false
}
