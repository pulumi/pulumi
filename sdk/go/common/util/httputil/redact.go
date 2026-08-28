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

package httputil

import (
	"net/url"
	"strings"
)

var sensitiveQueryKeys = map[string]bool{
	"sig":                  true, // Azure SAS signature
	"signature":            true,
	"x-amz-signature":      true,
	"x-amz-credential":     true,
	"x-amz-security-token": true,
	"x-goog-signature":     true,
	"x-goog-credential":    true,
	"awsaccesskeyid":       true,
	"token":                true,
	"access_token":         true,
}

func URLSecrets(raw string) []string {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	var secrets []string
	if u.User != nil {
		if pw, ok := u.User.Password(); ok && pw != "" {
			secrets = append(secrets, pw)
		}
	}
	for k, vs := range u.Query() {
		if !sensitiveQueryKeys[strings.ToLower(k)] {
			continue
		}
		for _, v := range vs {
			if v != "" {
				secrets = append(secrets, v)
			}
		}
	}
	return secrets
}

func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "[redacted url]"
	}
	if u.RawQuery != "" {
		q := u.Query()
		changed := false
		for k := range q {
			if sensitiveQueryKeys[strings.ToLower(k)] {
				q[k] = []string{"redacted"}
				changed = true
			}
		}
		if changed {
			u.RawQuery = q.Encode()
		}
	}
	return u.Redacted()
}
