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

// Package ocikms implements the OCI Key Management driver for Go Cloud Secrets.
package ocikms

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"gocloud.dev/secrets"
)

const Scheme = "ocikms"

func init() {
	secrets.DefaultURLMux().RegisterKeeper(Scheme, new(URLOpener))
}

type URLOpener struct{}

func (o *URLOpener) OpenKeeperURL(ctx context.Context, u *url.URL) (*secrets.Keeper, error) {
	if u.Scheme != Scheme || u.Host == "" || u.User != nil || u.Opaque != "" ||
		u.Fragment != "" || strings.TrimPrefix(u.Path, "/") == "" {
		return nil, errors.New("ocikms URL requires a crypto endpoint and key reference without credentials")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, errors.New("invalid ocikms URL query")
	}
	for name, values := range query {
		if name != "algo" && name != "conffile" && name != "profile" {
			return nil, errors.New("unsupported ocikms URL query parameter")
		}
		if len(values) != 1 || values[0] == "" {
			return nil, errors.New("ocikms URL parameters require one nonempty value")
		}
	}
	algo := query.Get("algo")
	if algo == "" {
		algo = "AES_256_GCM"
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	provider, err := newGCConfigurationProvider(u)
	if err != nil {
		return nil, err
	}
	return OpenKeeper(ctx, "https://"+u.Host, strings.TrimPrefix(u.Path, "/"), algo, provider)
}
