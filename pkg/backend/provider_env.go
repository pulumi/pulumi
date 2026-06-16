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

package backend

import "github.com/pulumi/pulumi/sdk/v3/go/common/env"

// ResourceProviderCredentialEnv builds the API-address and access-token environment injected into
// resource provider plugins. It returns nil when either is missing, as for a non-cloud login.
func ResourceProviderCredentialEnv(cloudURL, accessToken string) map[string]string {
	if cloudURL == "" || accessToken == "" {
		return nil
	}
	return map[string]string{
		env.APIURL.Var().Name():      cloudURL,
		env.AccessToken.Var().Name(): accessToken,
	}
}
