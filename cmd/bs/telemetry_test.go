package main

import "testing"

// TestRedactSecrets covers literal Telemetry credentials and overlap, empty,
// case-sensitive, repeated-match, and replacement-marker behavior.
func TestRedactSecrets(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		token    string
		username string
		password string
		want     string
	}{
		{
			name:  "empty text",
			token: "token", username: "user", password: "pass",
		},
		{
			name: "all empty leaves derived colon encoding alone",
			text: "unrelated Og== text", want: "unrelated Og== text",
		},
		{
			name: "token only with both SQL credentials empty",
			text: "token Og== token", token: "token",
			want: "[REDACTED] Og== [REDACTED]",
		},
		{
			name: "empty token",
			text: "user pass dXNlcjpwYXNz", username: "user", password: "pass",
			want: "[REDACTED] [REDACTED] [REDACTED]",
		},
		{
			name: "empty username includes Basic auth",
			text: "pass OnBhc3M=", password: "pass",
			want: "[REDACTED] [REDACTED]",
		},
		{
			name: "empty password includes Basic auth",
			text: "user dXNlcjo=", username: "user",
			want: "[REDACTED] [REDACTED]",
		},
		{
			name: "longer password before token substring",
			text: "short-long short short-long", token: "short", password: "short-long",
			want: "[REDACTED] [REDACTED] [REDACTED]",
		},
		{
			name: "longer Basic auth before token substring",
			text: "dXNlcjpwYXNz dXN", token: "dXN", username: "user", password: "pass",
			want: "[REDACTED] [REDACTED]",
		},
		{
			name: "longer match starts inside shorter match",
			text: "abcdef ab", token: "ab", password: "bcdef",
			want: "a[REDACTED] [REDACTED]",
		},
		{
			name: "duplicate secrets",
			text: "same c2FtZTpzYW1l same", token: "same", username: "same", password: "same",
			want: "[REDACTED] [REDACTED] [REDACTED]",
		},
		{
			name:  "case sensitive",
			text:  "TOKEN USER PASS DXNLCJPWYXNZ token user pass dXNlcjpwYXNz",
			token: "token", username: "user", password: "pass",
			want: "TOKEN USER PASS DXNLCJPWYXNZ [REDACTED] [REDACTED] [REDACTED] [REDACTED]",
		},
		{
			name: "no matching secret",
			text: "unrelated diagnostic text\n", token: "token", username: "user", password: "pass",
			want: "unrelated diagnostic text\n",
		},
		{
			name:  "every secret occurrence",
			text:  "token|user|pass|dXNlcjpwYXNz|token|user|pass|dXNlcjpwYXNz",
			token: "token", username: "user", password: "pass",
			want: "[REDACTED]|[REDACTED]|[REDACTED]|[REDACTED]|[REDACTED]|[REDACTED]|[REDACTED]|[REDACTED]",
		},
		{
			name: "literal metacharacters",
			text: "a.* aXYZ [user] p$", token: "a.*", username: "[user]", password: "p$",
			want: "[REDACTED] aXYZ [REDACTED] [REDACTED]",
		},
		{
			name: "replacement marker is not processed again",
			text: "user REDACTED pass", token: "REDACTED", username: "user", password: "pass",
			want: "[REDACTED] [REDACTED] [REDACTED]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactSecrets(tc.text, tc.token, tc.username, tc.password); got != tc.want {
				t.Errorf("redactSecrets() = %q, want %q", got, tc.want)
			}
		})
	}
}
