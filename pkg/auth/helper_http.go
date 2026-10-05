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

package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

func newHelperHTTPAuth(backendURL string, initial *credentialhelper.Response, run runHelperFunc) *HTTPAuth {
	suppliesToken := initial.AccessToken != ""
	credentials := func(response *credentialhelper.Response) httpCredentials {
		return httpCredentials{
			accessToken: response.AccessToken, headers: response.Headers, expiresAt: response.ExpiresAt,
		}
	}
	a := NewHTTPAuth(backendURL, "", nil)
	a.current = credentials(initial)
	a.refresh = func(ctx context.Context, reason refreshReason) (httpCredentials, error) {
		logging.V(7).Infof("Refreshing credential helper HTTP credentials (%s)", reason)
		response, err := run(ctx, credentialhelper.Request{
			Reason: credentialhelper.Reason(reason), SelectedBackendURL: backendURL,
		})
		switch {
		case err != nil:
		case !response.HasHTTPCredentials():
			err = errors.New("helper returned no HTTP credentials")
		case suppliesToken && response.AccessToken == "":
			err = errors.New("helper must continue supplying an accessToken")
		case !suppliesToken && response.AccessToken != "":
			err = errors.New("helper must not start supplying an accessToken on refresh")
		}
		if err != nil {
			return httpCredentials{}, fmt.Errorf("refreshing credential helper credentials: %w", err)
		}
		redactHelperCredentials(response)
		return credentials(response), nil
	}
	return a
}
