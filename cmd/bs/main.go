// Command bs is a read-only CLI for the Better Stack Uptime API.
//
// Milestone 001 delivers the installable skeleton and the three-level help
// surface only. No invocation performs a network request and no invocation
// reads a credential. Help forms exit 0; every other argument vector exits 1
// with a readable error on stderr.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes one CLI invocation and returns the process exit code. stdout
// and stderr are injected so tests observe both channels without a subprocess.
func run(args []string, stdout, stderr io.Writer) int {
	if help, ok := helpFor(args); ok {
		fmt.Fprint(stdout, help)
		return 0
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
	case len(args) == 2 && args[0] == "incidents" && args[1] == "--help":
		return incidentsHelp, true
	case len(args) == 2 && args[0] == "monitors" && args[1] == "--help":
		return monitorsHelp, true
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
	case "incidents", "monitors":
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
		return fmt.Sprintf("bs: %q is not implemented in this milestone; %s.\n", command, commandHint)
	}

	// get and timeline take exactly one positional target.
	placeholder := "<id-or-url>"
	if resource == "monitors" {
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
	case "monitors":
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
