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

package awssetup

import (
	"errors"
	"regexp"
	"strings"
)

// ssoStartURLPattern is the shape of an AWS IAM Identity Center (AWS SSO) access-portal
// "start" URL: HTTPS, on a subdomain of awsapps.com, amazonaws.com, or awsapps.cn, with an
// optional path, query, or fragment. The Pulumi Cloud console enforces the same rule client-side;
// keep the two in sync.
var ssoStartURLPattern = regexp.MustCompile(
	`(?i)^https://(?:[a-z0-9-]+\.)+(?:awsapps\.com|amazonaws\.com|awsapps\.cn)(?:[/?#]\S*)?$`,
)

// ErrInvalidSSOStartURL is returned by ValidateSSOStartURL. Its message is user-facing.
var ErrInvalidSSOStartURL = errors.New(
	"invalid SSO start URL: expected an HTTPS URL on an awsapps.com, amazonaws.com, or awsapps.cn domain, " +
		"for example https://d-xxxxxxxxxx.awsapps.com/start",
)

// ValidateSSOStartURL rejects start URLs that cannot be an AWS IAM Identity Center access-portal
// URL before they reach the SSO OIDC API, whose own rejection is far less descriptive.
func ValidateSSOStartURL(startURL string) error {
	if !ssoStartURLPattern.MatchString(strings.TrimSpace(startURL)) {
		return ErrInvalidSSOStartURL
	}
	return nil
}
