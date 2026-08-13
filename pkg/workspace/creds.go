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
	"errors"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// CloudURLSource identifies where the current cloud URL was configured. Knowing this
// matters when a backend turns out to be unreachable, because the way to correct it
// differs by source: a URL from the environment or from the project file keeps winning
// over stored credentials, so neither `pulumi login` nor `pulumi logout` will change it.
type CloudURLSource string

const (
	// CloudURLSourceNone indicates that no cloud URL is configured anywhere.
	CloudURLSourceNone CloudURLSource = ""
	// CloudURLSourceEnv indicates the URL came from the PULUMI_BACKEND_URL environment variable.
	CloudURLSourceEnv CloudURLSource = "env"
	// CloudURLSourceProject indicates the URL came from the `backend` setting in the project file.
	CloudURLSourceProject CloudURLSource = "project"
	// CloudURLSourceCredentials indicates the URL came from the stored credentials file.
	CloudURLSourceCredentials CloudURLSource = "credentials"
)

// GetCurrentCloudURL returns the URL of the cloud we are currently connected to. This may be empty if we
// have not logged in. Note if PULUMI_BACKEND_URL is set, the corresponding value is returned
// instead irrespective of the backend for current project or stored credentials.
func GetCurrentCloudURL(ws Context, e env.Env, project *workspace.Project) (string, error) {
	url, _, err := GetCurrentCloudURLWithSource(ws, e, project)
	return url, err
}

// GetCurrentCloudURLWithSource behaves like [GetCurrentCloudURL], and additionally reports
// which of the three sources the URL was read from.
func GetCurrentCloudURLWithSource(
	ws Context, e env.Env, project *workspace.Project,
) (string, CloudURLSource, error) {
	// Allow PULUMI_BACKEND_URL to override the current cloud URL selection
	if backend := e.GetString(env.BackendURL); backend != "" {
		return backend, CloudURLSourceEnv, nil
	}

	if project != nil && project.Backend != nil && project.Backend.URL != "" {
		return project.Backend.URL, CloudURLSourceProject, nil
	}

	creds, err := ws.GetStoredCredentials()
	if err != nil {
		return "", CloudURLSourceNone, err
	}
	if creds.Current == "" {
		return "", CloudURLSourceNone, nil
	}
	return creds.Current, CloudURLSourceCredentials, nil
}

// GetCurrentCloudURLWithAgentFallback returns the active cloud URL, using
// shared temporary agent credentials when an agent cannot read the default
// credentials.
func GetCurrentCloudURLWithAgentFallback(ws Context, e env.Env, project *workspace.Project) (string, error) {
	url, err := GetCurrentCloudURL(ws, e, project)
	if err == nil {
		return url, nil
	}

	if !workspace.AgentCredentialsFallbackEnabled() {
		logging.V(7).Infof("Could not get cloud URL from default credentials without agent fallback: %v", err)
		return "", err
	}

	agent := agentdetect.Detect(os.Getenv)
	logging.V(7).Infof(
		"Could not get cloud URL from default credentials in agent mode (%s); checking shared agent credentials: %v",
		agent, err)
	agentCreds, agentErr := workspace.GetAgentStoredCredentials()
	if agentErr != nil {
		return "", fmt.Errorf("could not get cloud url from agent credentials: %w", errors.Join(err, agentErr))
	}
	if agentCreds.Current != "" {
		logging.V(7).Infof("Using current cloud URL %q from shared agent credentials", agentCreds.Current)
	} else {
		logging.V(7).Infof("No current cloud URL found in shared agent credentials")
	}

	return agentCreds.Current, nil
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
