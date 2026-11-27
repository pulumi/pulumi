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

	mapset "github.com/deckarep/golang-set/v2"
)

var sensitiveQueryKeys = mapset.NewSet(
	"sig", // Azure SAS signature
	"signature",
	"x-amz-signature",
	"x-amz-credential",
	"x-amz-security-token",
	"x-goog-signature",
	"x-goog-credential",
	"awsaccesskeyid",
	"token",
	"access_token",
)

// URLSecrets returns the secrets embedded in raw: a userinfo password and the
// values of any sensitive query parameter. Each is returned both decoded and as
// written, because callers register the result with
// logging.AddGlobalSecretFilter and then log the URL verbatim, where a secret
// needing percent-escaping only ever appears escaped.
func URLSecrets(raw string) []string {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	var secrets []string
	add := func(decoded, escaped string) {
		if decoded != "" {
			secrets = append(secrets, decoded)
		}
		if escaped != "" && escaped != decoded {
			secrets = append(secrets, escaped)
		}
	}
	if u.User != nil {
		if pw, ok := u.User.Password(); ok {
			_, escaped, _ := strings.Cut(rawUserinfo(raw), ":")
			add(pw, escaped)
		}
	}
	// Walk the undecoded query rather than u.Query(), which has already
	// unescaped every value.
	for pair := range strings.SplitSeq(u.RawQuery, "&") {
		name, escaped, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		key, err := url.QueryUnescape(name)
		if err != nil {
			key = name
		}
		if !sensitiveQueryKeys.Contains(strings.ToLower(key)) {
			continue
		}
		decoded, err := url.QueryUnescape(escaped)
		if err != nil {
			decoded = ""
		}
		add(decoded, escaped)
	}
	return secrets
}

// rawUserinfo returns the userinfo of raw exactly as written, or "" when raw
// carries none. url.Parse splits the authority at its last "@" and the
// userinfo at its first ":", so this follows the same rules.
func rawUserinfo(raw string) string {
	_, rest, ok := strings.Cut(raw, "//")
	if !ok {
		return ""
	}
	authority := rest
	if i := strings.IndexAny(authority, "/?#"); i >= 0 {
		authority = authority[:i]
	}
	i := strings.LastIndex(authority, "@")
	if i < 0 {
		return ""
	}
	return authority[:i]
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
			if sensitiveQueryKeys.Contains(strings.ToLower(k)) {
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
