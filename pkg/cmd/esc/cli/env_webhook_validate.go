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
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

const (
	webhookFormatRaw               = "raw"
	webhookFormatSlack             = "slack"
	webhookFormatMSTeams           = "ms_teams"
	webhookFormatPulumiDeployments = "pulumi_deployments"

	// removeSecretSentinel is the value the service interprets as "delete the
	// stored secret" on a webhook PATCH. Sending an empty string in the PATCH
	// body leaves the secret unchanged, so this sentinel is the only way to
	// clear it without recreating the webhook.
	removeSecretSentinel = "__remove-secret"

	// webhookNameMaxLen mirrors the backend column width; names longer than
	// this are rejected by the API.
	webhookNameMaxLen = 32
)

var webhookFormats = []string{
	webhookFormatRaw,
	webhookFormatSlack,
	webhookFormatMSTeams,
	webhookFormatPulumiDeployments,
}

var (
	webhookNameRE             = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	pulumiDeploymentsTargetRE = regexp.MustCompile(`^[^\s/]+/[^\s/]+$`)
)

func validateWebhookName(name string) error {
	if name == "" {
		return errors.New("webhook name cannot be empty")
	}
	if len(name) > webhookNameMaxLen {
		return fmt.Errorf("webhook name must be at most %d characters", webhookNameMaxLen)
	}
	if !webhookNameRE.MatchString(name) {
		return fmt.Errorf("webhook name %q must only contain letters, digits, '.', '_', or '-'", name)
	}
	return nil
}

func validateWebhookFormat(format string) error {
	if slices.Contains(webhookFormats, format) {
		return nil
	}
	return fmt.Errorf("invalid --format %q: must be one of %s", format, strings.Join(webhookFormats, ", "))
}

// validateWebhookURL checks the payload URL against the rules the backend will
// apply for the given format. An empty format means "no coupling check" (used
// from `edit` when --url is changed but --format is not).
func validateWebhookURL(format, payloadURL string) error {
	if payloadURL == "" {
		return errors.New("--url cannot be empty")
	}
	switch format {
	case webhookFormatSlack:
		if !strings.HasPrefix(payloadURL, "https://hooks.slack.com/") {
			return errors.New("slack webhooks require a --url starting with https://hooks.slack.com/")
		}
	case webhookFormatPulumiDeployments:
		if !pulumiDeploymentsTargetRE.MatchString(payloadURL) {
			return errors.New("pulumi_deployments webhooks require --url to be of the form <project>/<stack>")
		}
	default:
		u, err := url.Parse(payloadURL)
		if err != nil {
			return fmt.Errorf("invalid --url %q: %w", payloadURL, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("--url %q must use http or https", payloadURL)
		}
		if u.Host == "" {
			return fmt.Errorf("--url %q is missing a host", payloadURL)
		}
	}
	return nil
}
