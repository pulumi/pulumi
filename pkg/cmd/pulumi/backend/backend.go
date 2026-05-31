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

package backend

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// BackendInstance is used to inject a backend mock from tests.
var BackendInstance backend.Backend

// DisableIntegrityCheckingFlag is the name of the persistent root flag that disables checkpoint state
// integrity verification.
const DisableIntegrityCheckingFlag = "disable-integrity-checking"

// DisableIntegrityChecking returns the value of the --disable-integrity-checking persistent flag for the
// given command. The flag is registered on the root command and inherited by all subcommands.
func DisableIntegrityChecking(cmd *cobra.Command) bool {
	v, err := cmd.Flags().GetBool(DisableIntegrityCheckingFlag)
	if err != nil {
		// The flag is registered on the root command as a persistent flag, so it is always present on
		// subcommands. If it is somehow missing we default to enforcing integrity checking, which is the safe
		// behavior.
		return false
	}
	return v
}

func IsDIYBackend(ws pkgWorkspace.Context, opts display.Options) (bool, error) {
	if BackendInstance != nil {
		return false, nil
	}

	// Try to read the current project
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return false, err
	}

	url, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
	if err != nil {
		return false, fmt.Errorf("could not get cloud url: %w", err)
	}

	return diy.IsDIYBackendURL(url), nil
}

func NonInteractiveCurrentBackend(
	ctx context.Context, ws pkgWorkspace.Context, lm LoginManager, project *workspace.Project,
) (backend.Backend, error) {
	if BackendInstance != nil {
		return BackendInstance, nil
	}

	url, err := getCurrentCloudURL(ws, project)
	if err != nil {
		return nil, err
	}
	logging.V(7).Infof("Current cloud URL: %q", url)

	// Only set current if we don't currently have a cloud URL set.
	return lm.Current(ctx, ws, cmdutil.Diag(), url, project, url == "")
}

func CurrentBackend(
	ctx context.Context, ws pkgWorkspace.Context, lm LoginManager, project *workspace.Project,
	opts display.Options,
) (backend.Backend, error) {
	if BackendInstance != nil {
		return BackendInstance, nil
	}

	url, err := getCurrentCloudURL(ws, project)
	if err != nil {
		return nil, err
	}
	logging.V(7).Infof("Current cloud URL: %q", url)
	insecure := pkgWorkspace.GetCloudInsecure(ws, url)

	// Only set current if we don't currently have a cloud URL set.
	return lm.Login(ctx, ws, cmdutil.Diag(), url, project, url == "", insecure, opts.Color)
}

// getCurrentCloudURL returns the active cloud URL, using the shared agent
// credentials as a fallback when an agent cannot read the default credentials.
func getCurrentCloudURL(ws pkgWorkspace.Context, project *workspace.Project) (string, error) {
	url, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
	if err == nil {
		return url, nil
	}

	agent := agentdetect.Detect(os.Getenv)
	if agent == "" || hasExplicitPulumiPathEnv() {
		logging.V(7).Infof("Could not get cloud URL from default credentials without agent fallback: %v", err)
		return "", fmt.Errorf("could not get cloud url: %w", err)
	}

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

// hasExplicitPulumiPathEnv reports whether the user explicitly selected a
// Pulumi credential or home path, disabling implicit agent fallback paths.
func hasExplicitPulumiPathEnv() bool {
	return os.Getenv(workspace.PulumiCredentialsPathEnvVar) != "" || os.Getenv(env.Home.Var().Name()) != ""
}
