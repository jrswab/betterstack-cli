package main

import (
	"encoding/base64"
	"sort"
	"strings"
)

// redactSecrets replaces non-empty Telemetry credentials and derived Basic
// authentication with [REDACTED]. Longer literal matches take priority,
// matching is case-sensitive, and replacement text is not scanned again.
// No current response path calls this helper.
func redactSecrets(text, telemetryToken, sqlUsername, sqlPassword string) string {
	secrets := []string{telemetryToken, sqlUsername, sqlPassword}
	if sqlUsername != "" || sqlPassword != "" {
		secrets = append(secrets, base64.StdEncoding.EncodeToString([]byte(sqlUsername+":"+sqlPassword)))
	}
	sort.SliceStable(secrets, func(i, j int) bool {
		return len(secrets[i]) > len(secrets[j])
	})
	parts := []string{text}
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		var next []string
		for _, part := range parts {
			next = append(next, strings.Split(part, secret)...)
		}
		parts = next
	}
	return strings.Join(parts, "[REDACTED]")
}
