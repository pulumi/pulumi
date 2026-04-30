// Copyright 2024, Pulumi Corporation.
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

package workspace

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// GetCurrentCloudURL returns the URL of the cloud we are currently connected to. This may be empty if we
// have not logged in. Note if PULUMI_BACKEND_URL is set, the corresponding value is returned
// instead irrespective of the backend for current project or stored credentials.
func GetCurrentCloudURL(ws Context, e env.Env, project *workspace.Project) (string, error) {
	// Allow PULUMI_BACKEND_URL to override the current cloud URL selection
	if backend := e.GetString(env.BackendURL); backend != "" {
		return backend, nil
	}

	var url string
	if project != nil {
		if project.Backend != nil {
			url = project.Backend.URL
		}
	}

	if url == "" {
		creds, err := ws.GetStoredCredentials()
		if err != nil {
			return "", err
		}
		url = creds.Current
	}

	return url, nil
}

// GetCloudInsecure returns if this cloud url is saved as one that should use insecure transport.
func GetCloudInsecure(ws Context, cloudURL string) bool {
	insecure := false
	creds, err := ws.GetStoredCredentials()
	// If this errors just assume insecure == false
	if err == nil {
		if account, has := creds.Accounts[cloudURL]; has {
			insecure = account.Insecure
		}
	}
	return insecure
}
