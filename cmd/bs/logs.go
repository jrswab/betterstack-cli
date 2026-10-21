package main

import (
	"fmt"
	"regexp"
	"strings"
)

var logsTablePattern = regexp.MustCompile(`^t[0-9]+_[a-z0-9_]+$`)

type logsSearchArgs struct {
	table  string
	cold   bool
	query  *string
	values map[string]string
}

func parseLogsSearchArgs(args []string) (logsSearchArgs, error) {
	parsed := logsSearchArgs{values: make(map[string]string)}
	for i := 0; i < len(args); i++ {
		flag := args[i]
		switch flag {
		case "--cold":
			parsed.cold = true
		case "--table", "--since", "--from", "--to", "--query", "--order", "--limit", "--timeout":
			if _, exists := parsed.values[flag]; exists {
				return logsSearchArgs{}, fmt.Errorf("duplicate %s", flag)
			}
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
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
		parsed.query = &value
	}
	return parsed, nil
}
