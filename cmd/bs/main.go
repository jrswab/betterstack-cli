// Command bs is a read-only CLI for the Better Stack Uptime and Telemetry APIs.
// Exact help forms return without reading credentials or sending requests.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// @anchor:telemetry:config
const defaultTelemetryRESTBaseURL = "https://telemetry.betterstack.com"

func main() {
	os.Exit(runWithAPI(os.Args[1:], os.Stdout, os.Stderr, defaultAPIBaseURL, newHTTPClient()))
}

// run keeps the 001 argv/writer seam for existing tests; it uses the
// production API endpoint and client.
func run(args []string, stdout, stderr io.Writer) int {
	return runWithAPI(args, stdout, stderr, defaultAPIBaseURL, newHTTPClient())
}

// runWithAPI preserves the Uptime injection interface for production and tests.
// Tests supply a loopback base URL and HTTP client. No flag, environment
// variable, or file changes the production endpoint or client at runtime.
// @anchor:uptime:router
func runWithAPI(args []string, stdout, stderr io.Writer, baseURL string, client *http.Client) int {
	return runWithEndpoints(args, stdout, stderr, baseURL, defaultTelemetryRESTBaseURL, "", client)
}

// runWithEndpoints checks exact help before dispatching commands to distinct endpoints.
// @anchor:uptime:router endpoints
func runWithEndpoints(args []string, stdout, stderr io.Writer, uptimeBaseURL, telemetryRESTBaseURL, telemetrySQLURL string, client *http.Client) int {
	if help, ok := helpFor(args); ok {
		fmt.Fprint(stdout, help)
		return 0
	}
	if len(args) >= 2 && args[0] == "monitors" && args[1] == "list" {
		return monitorsList(args[2:], stdout, stderr, uptimeBaseURL, client)
	}
	if len(args) >= 2 && args[0] == "monitors" && args[1] == "get" {
		return monitorsGet(args[2:], stdout, stderr, uptimeBaseURL, client)
	}
	if len(args) >= 2 && args[0] == "incidents" && args[1] == "get" {
		return incidentsGet(args[2:], stdout, stderr, uptimeBaseURL, client)
	}
	if len(args) >= 2 && args[0] == "incidents" && args[1] == "timeline" {
		return incidentsTimeline(args[2:], stdout, stderr, uptimeBaseURL, client)
	}
	if len(args) >= 2 && args[0] == "incidents" && args[1] == "list" {
		return incidentsList(args[2:], stdout, stderr, uptimeBaseURL, client)
	}
	if len(args) >= 2 && args[0] == "sources" && args[1] == "list" {
		return sourcesList(args[2:], stdout, stderr, telemetryRESTBaseURL, client)
	}
	if len(args) >= 2 && args[0] == "sources" && args[1] == "get" {
		return sourcesGet(args[2:], stdout, stderr, telemetryRESTBaseURL, client)
	}
	if len(args) >= 2 && args[0] == "logs" && args[1] == "search" {
		return logsSearch(args[2:], stdout, stderr, telemetrySQLURL, client)
	}
	fmt.Fprint(stderr, usageError(args))
	return 1
}

// helpFor returns the help text for exactly the recognized help argument
// vectors. Every other vector is not a help request. Each token must match
// exactly; no extra tokens are accepted (spec R5.1).
func helpFor(args []string) (string, bool) {
	switch {
	case len(args) == 1 && args[0] == "--help":
		return rootHelp, true
	case len(args) == 2 && args[0] == "sources" && args[1] == "--help":
		return sourcesHelp, true
	case len(args) == 3 && args[0] == "sources" && args[1] == "list" && args[2] == "--help":
		return sourcesListHelp, true
	case len(args) == 3 && args[0] == "sources" && args[1] == "get" && args[2] == "--help":
		return sourcesGetHelp, true
	case len(args) == 2 && args[0] == "incidents" && args[1] == "--help":
		return incidentsHelp, true
	case len(args) == 2 && args[0] == "monitors" && args[1] == "--help":
		return monitorsHelp, true
	case len(args) == 3 && args[0] == "incidents" && args[1] == "list" && args[2] == "--help":
		return incidentsListHelp, true
	case len(args) == 3 && args[0] == "incidents" && args[1] == "get" && args[2] == "--help":
		return incidentsGetHelp, true
	case len(args) == 3 && args[0] == "incidents" && args[1] == "timeline" && args[2] == "--help":
		return incidentsTimelineHelp, true
	case len(args) == 3 && args[0] == "monitors" && args[1] == "list" && args[2] == "--help":
		return monitorsListHelp, true
	case len(args) == 3 && args[0] == "monitors" && args[1] == "get" && args[2] == "--help":
		return monitorsGetHelp, true
	}
	return "", false
}

// usageError builds the single readable error for an invalid argument vector.
// Every message names what was wrong and points to the relevant --help level.
func usageError(args []string) string {
	const rootHint = "run 'bs --help' for usage"

	if len(args) == 0 {
		return fmt.Sprintf("bs: no command given; %s.\n", rootHint)
	}

	switch args[0] {
	case "--help":
		return fmt.Sprintf("bs: --help takes no further arguments; %s.\n", rootHint)
	case "incidents", "monitors", "sources", "logs":
		return resourceUsageError(args[0], args[1:])
	default:
		if strings.HasPrefix(args[0], "-") {
			return fmt.Sprintf("bs: unknown option %q; %s.\n", args[0], rootHint)
		}
		return fmt.Sprintf("bs: unknown resource %q; %s.\n", args[0], rootHint)
	}
}

// resourceUsageError handles the invalid vectors whose first token is a known
// resource. rest is the argument vector after the resource.
func resourceUsageError(resource string, rest []string) string {
	resourceHint := fmt.Sprintf("run 'bs %s --help' for usage", resource)

	if len(rest) == 0 {
		return fmt.Sprintf("bs: missing action for %q; %s.\n", resource, resourceHint)
	}

	action := rest[0]
	if !knownAction(resource, action) {
		if strings.HasPrefix(action, "-") {
			return fmt.Sprintf("bs: unknown option %q; %s.\n", action, resourceHint)
		}
		return fmt.Sprintf("bs: unknown action %q for %q; %s.\n", action, resource, resourceHint)
	}

	command := resource + " " + action
	commandHint := fmt.Sprintf("run 'bs %s --help' for usage", command)

	// A --help token anywhere after the recognized help position is a
	// misplaced help flag, not a request for this command's help.
	if contains(rest[1:], "--help") {
		return fmt.Sprintf("bs: --help must follow the command exactly; %s.\n", commandHint)
	}

	if action == "list" {
		if len(rest) > 1 {
			return fmt.Sprintf("bs: unexpected argument %q for %q; %s.\n", rest[1], command, commandHint)
		}
		// A bare `monitors list` or `incidents list` is routed before this
		// function; report a defensive error rather than claiming the
		// implemented command is not.
		return fmt.Sprintf("bs: unexpected argument vector for %q; %s.\n", command, commandHint)
	}

	// get and timeline take exactly one positional target.
	placeholder := "<id-or-url>"
	if resource == "monitors" || resource == "sources" {
		placeholder = "<id>"
	}
	switch {
	case len(rest) == 1:
		return fmt.Sprintf("bs: missing %s for %q; %s.\n", placeholder, command, commandHint)
	case len(rest) > 2:
		return fmt.Sprintf("bs: too many arguments for %q; %s.\n", command, commandHint)
	default:
		return fmt.Sprintf("bs: %q is not implemented in this milestone; %s.\n", command, commandHint)
	}
}

func knownAction(resource, action string) bool {
	switch resource {
	case "incidents":
		return action == "list" || action == "get" || action == "timeline"
	case "logs":
		return action == "search"
	case "monitors", "sources":
		return action == "list" || action == "get"
	default:
		return false
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
