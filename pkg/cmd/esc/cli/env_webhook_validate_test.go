// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"strings"
	"testing"
)

func TestValidateWebhookName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		ok   bool
	}{
		{"empty", "", false},
		{"basic", "deploy", true},
		{"dots-dashes-underscores", "a.b-c_d", true},
		{"slash", "a/b", false},
		{"space", "a b", false},
		{"max-len", strings.Repeat("a", webhookNameMaxLen), true},
		{"too-long", strings.Repeat("a", webhookNameMaxLen+1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateWebhookName(tc.in)
			if tc.ok && err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected error for %q", tc.in)
			}
		})
	}
}

func TestValidateWebhookFormat(t *testing.T) {
	t.Parallel()
	for _, f := range webhookFormats {
		if err := validateWebhookFormat(f); err != nil {
			t.Fatalf("expected %q to be valid: %v", f, err)
		}
	}
	if err := validateWebhookFormat("json"); err == nil {
		t.Fatal("expected json to be rejected")
	}
	if err := validateWebhookFormat(""); err == nil {
		t.Fatal("expected empty format to be rejected")
	}
}

func TestValidateWebhookURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		format string
		url    string
		ok     bool
	}{
		{"raw-https", webhookFormatRaw, "https://example.com/hook", true},
		{"raw-http", webhookFormatRaw, "http://example.com/hook", true},
		{"raw-no-scheme", webhookFormatRaw, "example.com/hook", false},
		{"raw-ftp", webhookFormatRaw, "ftp://example.com/hook", false},
		{"raw-empty", webhookFormatRaw, "", false},
		{"slack-ok", webhookFormatSlack, "https://hooks.slack.com/services/abc", true},
		{"slack-wrong-host", webhookFormatSlack, "https://example.com/hook", false},
		{"slack-http", webhookFormatSlack, "http://hooks.slack.com/services/abc", false},
		{"deployments-ok", webhookFormatPulumiDeployments, "myproject/mystack", true},
		{"deployments-extra-slash", webhookFormatPulumiDeployments, "myproject/mystack/foo", false},
		{"deployments-no-slash", webhookFormatPulumiDeployments, "myproject", false},
		{"deployments-space", webhookFormatPulumiDeployments, "my project/stack", false},
		{"msteams-https", webhookFormatMSTeams, "https://outlook.office.com/webhook/abc", true},
		{"empty-format-https", "", "https://example.com/hook", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateWebhookURL(tc.format, tc.url)
			if tc.ok && err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected error for format=%q url=%q", tc.format, tc.url)
			}
		})
	}
}
